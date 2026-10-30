#!/usr/bin/env python3
"""Keep a local, immutable Compose recovery point until a rollout is healthy."""

import argparse
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import time
from urllib.parse import urljoin, urlsplit, urlunsplit


def run(*args, capture=False):
    return subprocess.run(args, check=True, text=True, stdout=subprocess.PIPE if capture else None).stdout


def compose(*args, snapshot=False, capture=False):
    filename = "guards.json" if snapshot == "guards" else "compose.json"
    files = ["-f", str(Path.cwd() / ".rollback" / filename)] if snapshot else []
    return run("docker", "compose", "--project-directory", str(Path.cwd()), *files, *args, capture=capture)


def inspect(container):
    return json.loads(run("docker", "inspect", container, capture=True))[0]


def service_container(service, snapshot=False):
    # `compose ps -a` includes `compose run` guards: never mistake those for the release.
    containers = compose("ps", "-aq", service, snapshot=snapshot, capture=True).split()
    for container in containers:
        labels = inspect(container)["Config"]["Labels"]
        if labels.get("com.docker.compose.oneoff", "false").lower() != "true":
            return container
    return ""


def healthy_url(url):
    return subprocess.run(["curl", "-kfsS", "--connect-timeout", "2", "--max-time", "5", url],
                          stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0


def web_healthy(container, service):
    info = inspect(container)
    if not info["State"]["Running"]:
        return False
    port, path = (8080, "/health") if service == "backend" else (3000, "/")
    # The host can probe the bridge address, including scratch images with no curl.
    addresses = [network["IPAddress"] for network in info["NetworkSettings"]["Networks"].values()]
    return any(address and healthy_url(f"http://{address}:{port}{path}") for address in addresses)


def wait_healthy(health_url, snapshot=False):
    print("Waiting for services to become healthy...", flush=True)
    for _ in range(30):
        try:
            ready = True
            for service in ("backend", "frontend"):
                container = service_container(service, snapshot=snapshot)
                ready = bool(container) and web_healthy(container, service) and ready
            if ready and healthy_url(health_url):
                print("Healthcheck passed!", flush=True)
                return True
        except subprocess.CalledProcessError:
            pass
        time.sleep(2)
    print("ERROR: Post-deploy healthcheck timed out!", flush=True)
    return False


def capture(health_url):
    os.umask(0o077)
    state = Path(".rollback")
    routing_record = state / "routing-path"
    if routing_record.exists() and Path(routing_record.read_text()).exists():
        raise RuntimeError("Unresolved rollover routes: retain recovery state and recover before deploying again")
    previous = Path("docker-compose.yml").exists()
    if previous and not wait_healthy(health_url):
        raise RuntimeError("Refusing deployment: current release is not healthy; recovery state retained")
    if state.exists():
        shutil.rmtree(state)
    state.mkdir(mode=0o700)
    if not previous:
        print("First deployment: no previous release available for rollback.", flush=True)
        return
    config = json.loads(compose("config", "--format", "json", capture=True))
    containers_before = {}
    for service, spec in config["services"].items():
        container = service_container(service)
        if not container:
            raise RuntimeError(f"Cannot capture previous image for {service}")
        # Container .Image is the immutable local sha256 ID, even after a tag moves.
        actual = inspect(container)
        containers_before[service] = actual["Id"]
        spec["image"] = actual["Image"]
        # SSH receives the NEW release's secrets; restore the running release's values.
        spec["environment"] = dict(item.split("=", 1) for item in actual["Config"]["Env"])
        spec["labels"] = {key: value for key, value in actual["Config"]["Labels"].items()
                          if not key.startswith("com.docker.compose.")}
        spec.pop("build", None)
        spec["pull_policy"] = "never"
    # Prevent a second Compose interpolation of literal dollars in resolved secrets.
    (state / "compose.json").write_text(json.dumps(config).replace("$", "$$"))
    (state / "containers.json").write_text(json.dumps(containers_before))
    files = state / "files"
    files.mkdir()
    for source in Path.cwd().iterdir():
        if source.is_file():
            shutil.copy2(source, files / source.name)
    print("Saved immutable images and resolved routing/config in .rollback (private).", flush=True)


def guard_config():
    config = json.loads(Path(".rollback/compose.json").read_text())
    for service in ("backend", "frontend"):
        # File-provider routes can be withdrawn before their live endpoints are removed.
        config["services"][service]["labels"] = {"traefik.enable": "false"}
    frontend_env = config["services"]["frontend"]["environment"]
    if "BACKEND_API_URL" in frontend_env:
        url = urlsplit(frontend_env["BACKEND_API_URL"])
        backend = config["services"]["backend"]
        if url.hostname in ("backend", backend.get("hostname"), backend.get("container_name")):
            upstream = url.netloc.replace(url.hostname, f"gate-rollover-backend-{os.getpid()}", 1)
            frontend_env["BACKEND_API_URL"] = urlunsplit(url._replace(netloc=upstream))
    Path(".rollback/guards.json").write_text(json.dumps(config))


def install_guard_routes(guards):
    config = json.loads(Path(".rollback/compose.json").read_text())
    backend_networks = inspect(guards[0][0])["NetworkSettings"]["Networks"]
    provider = None
    for container in run("docker", "ps", "-q", capture=True).split():
        info = inspect(container)
        shared = set(backend_networks) & set(info["NetworkSettings"]["Networks"])
        for mount in info["Mounts"]:
            if shared and mount["Destination"] == "/etc/traefik/dynamic" and mount["Type"] == "bind":
                provider = (Path(mount["Source"]), shared)
    if not provider or not provider[0].is_dir():
        raise RuntimeError("Traefik must watch a host-writable directory mounted at /etc/traefik/dynamic")
    directory, networks = provider
    http = {"routers": {}, "services": {}, "middlewares": {}}
    for name, service in guards:
        addresses = inspect(name)["NetworkSettings"]["Networks"]
        address = next(addresses[network]["IPAddress"] for network in networks if network in addresses)
        upstream = f"rollover-{config['name']}-{service}"
        marker = upstream + "-marker"
        port, path = (8080, "/health") if service == "backend" else (3000, "/")
        http["services"][upstream] = {"loadBalancer": {
            "servers": [{"url": f"http://{address}:{port}"}],
            "healthCheck": {"path": path, "interval": "2s", "timeout": "1s"}}}
        http["middlewares"][marker] = {"headers": {"customResponseHeaders": {"X-Gate-Rollover": service}}}
        labels = {key: value.replace("$$", "$") for key, value in config["services"][service]["labels"].items()}
        for key, rule in labels.items():
            match = re.fullmatch(r"traefik\.http\.routers\.([^.]+)\.rule", key)
            if not match:
                continue
            prefix = key.removesuffix("rule")
            router = {"rule": rule, "service": upstream,
                      "priority": int(labels.get(prefix + "priority", len(rule))) + 1000,
                      "middlewares": [item if "@" in item else item + "@docker"
                                      for item in labels.get(prefix + "middlewares", "").split(",") if item] + [marker]}
            if prefix + "entrypoints" in labels:
                router["entryPoints"] = labels[prefix + "entrypoints"].split(",")
            if labels.get(prefix + "tls") == "true":
                router["tls"] = {}
                if prefix + "tls.certresolver" in labels:
                    router["tls"]["certResolver"] = labels[prefix + "tls.certresolver"]
            http["routers"][f"{upstream}-{match[1]}"] = router
    routing_path = directory / f"gate-rollover-{config['name']}.json"
    if routing_path.exists():
        raise RuntimeError("Existing rollover routing requires operator recovery before another deployment")
    Path(".rollback/routing-path").write_text(str(routing_path))
    write_routes(routing_path, json.dumps({"http": http}))
    return routing_path


def write_routes(path, content):
    temporary = path.with_suffix(".tmp")
    temporary.write_text(content)
    temporary.chmod(0o644)  # Traefik can run as another user; routes contain no credentials.
    temporary.replace(path)


def guard_routes_ready(health_url, expected=True):
    for service, url in (("backend", health_url), ("frontend", urljoin(health_url, "/"))):
        response = subprocess.run(["curl", "-kfsS", "--max-time", "5", "-D", "-", "-o", "/dev/null", url],
                                  text=True, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
        marked = f"x-gate-rollover: {service}" in response.stdout.lower()
        if response.returncode or marked != expected:
            return False
    return True


def withdraw_guard_routes(path, health_url):
    content = path.read_text()
    path.unlink()
    for _ in range(30):
        if guard_routes_ready(health_url, expected=False):
            print("Traefik switched to the verified release; removing stable guards.", flush=True)
            return
        time.sleep(2)
    write_routes(path, content)
    raise RuntimeError("Traefik handoff failed: retaining guard routes and live instances for recovery")


def rollout(health_url):
    snapshot = Path(".rollback/compose.json").exists()
    guards = []
    routing_path = None
    success = False
    try:
        if snapshot:
            guard_config()
            for service in ("backend", "frontend"):
                name = f"gate-rollover-{service}-{os.getpid()}"
                guards.append((name, service))
                compose("run", "-d", "--no-deps", "--name", name, service, snapshot="guards")
            routing_path = install_guard_routes(guards)
            for _ in range(30):
                if all(web_healthy(name, service) for name, service in guards) and guard_routes_ready(health_url):
                    break
                time.sleep(2)
            else:
                raise RuntimeError("Stable routing guards did not become healthy")
        compose("pull", "--ignore-buildable")
        compose("up", "-d")
        if not wait_healthy(health_url):
            raise RuntimeError("New release failed health verification")
        if routing_path:
            withdraw_guard_routes(routing_path, health_url)
        success = True
        print("Deployment succeeded.", flush=True)
    except (subprocess.CalledProcessError, RuntimeError) as error:
        print(f"ERROR: Deployment failed: {error}", flush=True)
        subprocess.run(["docker", "compose", "ps"], check=False)
        if snapshot:
            print("ROLLBACK: restoring previous images and routing/config...", flush=True)
            for source in Path(".rollback/files").iterdir():
                shutil.copy2(source, Path.cwd() / source.name)
            try:
                before = json.loads(Path(".rollback/containers.json").read_text())
                changed = [service for service, container in before.items()
                           if service_container(service, snapshot=True) != container]
                if changed:
                    # Do not restart unchanged databases/storage just to change tag -> ID syntax.
                    compose("up", "-d", "--no-deps", "--pull", "never", *changed, snapshot=True)
                config = json.loads(Path(".rollback/compose.json").read_text())
                containers = run("docker", "ps", "-aq", "--filter",
                                 f"label=com.docker.compose.project={config['name']}", capture=True).split()
                for container in containers:
                    labels = inspect(container)["Config"]["Labels"]
                    if labels.get("com.docker.compose.service") not in before:
                        run("docker", "rm", "-f", container)
                if not wait_healthy(health_url, snapshot=True):
                    raise RuntimeError("Previous release did not recover")
                if routing_path:
                    withdraw_guard_routes(routing_path, health_url)
                success = True
                print("ROLLBACK SUCCEEDED: previous release healthy; deployment remains failed.", flush=True)
            except (subprocess.CalledProcessError, RuntimeError):
                print("ROLLBACK FAILED: preserving stable routing guards and .rollback for recovery.", flush=True)
                raise
        else:
            print("ROLLBACK UNAVAILABLE: no previous release (first deployment).", flush=True)
        raise
    finally:
        if success:
            for name, _ in guards:
                run("docker", "rm", "-f", name)
        # Never prune images, remove volumes, or roll back database migrations here.


if __name__ == "__main__":
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation", choices=("capture", "deploy"))
    parser.add_argument("--health-url", required=True)
    arguments = parser.parse_args()
    {"capture": capture, "deploy": rollout}[arguments.operation](arguments.health_url)
