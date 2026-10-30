"""Exercise deployment recovery against real Compose containers and Traefik."""

import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import threading
import time
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).resolve().parents[1] / "common/rollout.py"
spec = importlib.util.spec_from_file_location("rollout", SCRIPT)
rollout = importlib.util.module_from_spec(spec)
spec.loader.exec_module(rollout)


class DockerRolloutTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        for image in ("nginx:alpine", "traefik:v3.7", "python:3.11-slim"):
            result = subprocess.run(["docker", "image", "inspect", image],
                                    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            if result.returncode:
                rollout.run("docker", "pull", image)

    def setUp(self):
        self.previous = Path.cwd()
        self.temp = tempfile.TemporaryDirectory(prefix="gate-rollout-test-")
        os.chdir(self.temp.name)
        self.project = Path.cwd().name
        self.tag = self.project + ":release"
        self.addCleanup(self.cleanup)
        rollout.run("docker", "tag", "nginx:alpine", self.tag)
        config = {"name": self.project, "services": {}}
        for service, port in (("backend", 8080), ("frontend", 3000)):
            Path(service + ".conf").write_text(
                f'events {{}} http {{ server {{ listen {port}; location / {{ return 200 "stable"; }} }} }}')
            config["services"][service] = {
                "image": self.tag if service == "backend" else "nginx:alpine", "pull_policy": "never",
                "volumes": [f"./{service}.conf:/etc/nginx/nginx.conf:ro"],
                "environment": {"LITERAL": "secret$$dollar", "BACKEND_API_URL": "http://backend:8080"},
                "labels": {
                    "traefik.enable": "true",
                    "gate.rollout.test": self.project,
                    f"traefik.http.routers.{service}.rule": "PathPrefix(`/health`)" if service == "backend" else "PathPrefix(`/`)",
                    f"traefik.http.routers.{service}.service": service,
                    f"traefik.http.services.{service}.loadbalancer.server.port": str(port),
                },
            }
        config["services"]["traefik"] = {
            "image": "traefik:v3.7", "pull_policy": "never",
            "command": ["--providers.docker=true", "--providers.docker.exposedbydefault=false",
                        f"--providers.docker.constraints=Label(`gate.rollout.test`, `{self.project}`)",
                        "--entrypoints.web.address=:80"],
            "ports": ["127.0.0.1::80"],
            "volumes": ["/var/run/docker.sock:/var/run/docker.sock:ro"],
        }
        self.config = config
        self.write_config()
        rollout.compose("up", "-d")
        port = rollout.compose("port", "traefik", "80", capture=True).strip()
        self.url = f"http://{port}/health"
        self.assertTrue(rollout.wait_healthy(self.url))

    def write_config(self):
        Path("docker-compose.yml").write_text(json.dumps(self.config))

    def cleanup(self):
        containers = subprocess.check_output(
            ["docker", "ps", "-aq", "--filter", f"label=com.docker.compose.project={self.project}"], text=True).split()
        if containers:
            subprocess.run(["docker", "rm", "-f", *containers], check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["docker", "network", "rm", self.project + "_default"], check=False, stdout=subprocess.DEVNULL)
        subprocess.run(["docker", "image", "rm", self.tag], check=False, stdout=subprocess.DEVNULL)
        os.chdir(self.previous)
        self.temp.cleanup()

    def test_failed_release_restores_images_config_and_routes(self):
        original_config = Path("docker-compose.yml").read_bytes()
        unchanged = {service: rollout.compose("ps", "-q", service, capture=True).strip()
                     for service in ("frontend", "traefik")}
        original = rollout.inspect(rollout.compose("ps", "-q", "backend", capture=True).strip())["Image"]
        rollout.capture(self.url)
        self.assertEqual(0o700, Path(".rollback").stat().st_mode & 0o777)
        snapshot = json.loads(Path(".rollback/compose.json").read_text())
        self.assertEqual(original, snapshot["services"]["backend"]["image"])
        # A new image exits immediately and also changes routing. Old routes must survive.
        rollout.run("docker", "tag", "python:3.11-slim", self.tag)
        self.config["services"]["backend"]["command"] = ["python", "-c", "raise SystemExit(1)"]
        self.config["services"]["backend"]["labels"]["traefik.http.routers.backend.rule"] = "PathPrefix(`/changed`)"
        self.write_config()
        failures, samples = [], []
        stop = threading.Event()

        def probe():
            while not stop.wait(0.1):
                for url in (self.url, self.url.removesuffix("health")):
                    healthy = rollout.healthy_url(url)
                    samples.append(healthy)
                    if not healthy:
                        failures.append(url)

        monitor = threading.Thread(target=probe)
        monitor.start()
        try:
            with self.assertRaises(RuntimeError):
                rollout.rollout(self.url)
        finally:
            stop.set()
            monitor.join()
        self.assertGreater(len(samples), 50)
        self.assertFalse(failures, f"Traefik lost healthy routing: {len(failures)} failures; first: {failures[:5]}")
        guards = json.loads(Path(".rollback/guards.json").read_text())
        self.assertEqual(f"http://gate-rollover-backend-{os.getpid()}:8080",
                         guards["services"]["frontend"]["environment"]["BACKEND_API_URL"])
        self.assertEqual(original_config, Path("docker-compose.yml").read_bytes())
        restored = rollout.inspect(rollout.compose("ps", "-q", "backend", capture=True).strip())
        self.assertEqual(original, restored["Image"])
        for service, container in unchanged.items():
            self.assertEqual(container, rollout.compose("ps", "-q", service, capture=True).strip())
        self.assertIn("LITERAL=secret$dollar", restored["Config"]["Env"])
        self.assertEqual("PathPrefix(`/health`)", restored["Config"]["Labels"]["traefik.http.routers.backend.rule"])
        self.assertTrue(rollout.wait_healthy(self.url))
        self.assertEqual(3, len(rollout.compose("ps", "-aq", capture=True).split()))
        # The same path must also activate a healthy release and remove both guards.
        rollout.capture(self.url)
        self.config = json.loads(original_config)
        self.config["services"]["backend"]["image"] = "nginx:alpine"
        self.config["services"]["backend"]["environment"]["VERSION"] = "new-release"
        self.write_config()
        rollout.rollout(self.url)
        active = rollout.inspect(rollout.compose("ps", "-q", "backend", capture=True).strip())
        self.assertIn("VERSION=new-release", active["Config"]["Env"])
        self.assertEqual(3, len(rollout.compose("ps", "-aq", capture=True).split()))


class FailureHandlingTest(unittest.TestCase):
    def test_unhealthy_current_release_retains_previous_recovery_point(self):
        with tempfile.TemporaryDirectory() as directory:
            previous = Path.cwd()
            os.chdir(directory)
            try:
                Path("docker-compose.yml").write_text("{}")
                Path(".rollback").mkdir()
                Path(".rollback/compose.json").write_text("previous recovery point")
                with patch.object(rollout, "wait_healthy", return_value=False):
                    with self.assertRaises(RuntimeError):
                        rollout.capture("https://gate.test/health")
                self.assertEqual("previous recovery point", Path(".rollback/compose.json").read_text())
            finally:
                os.chdir(previous)

    def test_partial_startup_rolls_back_and_failed_rollback_keeps_guards(self):
        with tempfile.TemporaryDirectory() as directory:
            previous = Path.cwd()
            os.chdir(directory)
            try:
                Path(".rollback/files").mkdir(parents=True)
                Path(".rollback/compose.json").write_text('{"name": "test"}')
                Path(".rollback/containers.json").write_text('{"backend": "old"}')
                operations = []

                def compose(*args, **kwargs):
                    operations.append((args, kwargs))
                    if args[:2] == ("up", "-d"):
                        raise subprocess.CalledProcessError(1, "partial up")
                    return ""

                with patch.object(rollout, "compose", side_effect=compose), \
                     patch.object(rollout, "guard_config"), \
                     patch.object(rollout, "web_healthy", return_value=True), \
                     patch.object(rollout, "service_container", return_value="new"), \
                     patch.object(rollout, "guard_routes_ready", return_value=True), \
                     patch.object(rollout, "run") as run, \
                     patch.object(rollout.subprocess, "run"):
                    with self.assertRaises(subprocess.CalledProcessError):
                        rollout.rollout("https://gate.test/health")
                    run.assert_not_called()  # Neither guard may be removed on rollback failure.
                self.assertIn((("up", "-d", "--no-deps", "--pull", "never", "backend"), {"snapshot": True}), operations)
            finally:
                os.chdir(previous)


if __name__ == "__main__":
    unittest.main()
