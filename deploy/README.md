# Deployment

Gate runs on k3s and is deployed with Helm as two releases in the `gate` namespace:

| Release | Chart | Contents | Upgrades |
|---|---|---|---|
| `gate-infra` | `charts/gate-infra` | PostgreSQL, Redis, NATS, SeaweedFS (StatefulSets on node disk) | rarely; never rolled back with the app |
| `gate` | `charts/gate` | backend with a go-judge sidecar, migration Job, Traefik routes | every release |

`.github/workflows/deploy.yml` tags the images that CI tested as `release-<commit>`.
It then runs `helm upgrade --install gate --rollback-on-failure` with that tag:

1. A `pre-upgrade` Job runs `backend --migrate`. If it fails, nothing of the new release starts.
2. Backend pods roll one at a time (`maxUnavailable: 0`). A new pod must pass `/health` before an old one stops.
3. If the migration or the rollout fails within the timeout, Helm restores the previous release and the workflow fails. The previous pods serve throughout.
4. The workflow then checks `https://<host>/health` from outside. A failure there (route, certificate, DNS) fails the workflow but does not roll back.

A rolled-back release runs on the migrated schema, so migrations must follow
`docs/design/migrations.md`. To roll back by hand, run Deploy Production with the
commit of an earlier release, or run `helm -n gate rollback gate`.

## Layout

- `charts/`: the two charts. Their defaults fit one node.
- `secrets.sh`: writes the `gate-secrets` secret, used by the deploy and `task k3d:deploy`.
- `prod/`: the production backend config (`config.yaml`, mounted with `--set-file backend.config=…`, loaded by the config tests) and chart values (domain, sizing).
- `k3s/`: `install.sh` prepares a server once; the manifests are cluster bootstrap applied on every deploy. It configures the bundled Traefik (HTTPS redirect, Let's Encrypt, client IPs) and the headless `traefik-pods` service that `http.trusted_proxy` names.
- `k3d/`: the same files for the local cluster.
- `local/`: Docker Compose services for `task dev`.

## Server

Prepare a server once, as root:

```sh
sudo sh install.sh deployer   # deploy/k3s/install.sh copied to the server
```

The script:

- adds 2 GB of swap; the kubelet runs with `LimitedSwap`, so pods can use it too;
- closes port 6443 on the default-route interface with a boot-time unit;
- installs k3s (pinned version, installer from the same tag, without metrics-server);
- copies the kubeconfig to `~deployer/.kube/config`.

It is safe to run again and leaves Docker and its containers alone. The server also
needs DNS for the domain in `prod/gate.yaml`, and ports 80 and 443 open and free:
k3s's Traefik binds them, so nothing else on the host may hold them.

The deploy opens an SSH tunnel to the server with the `SSH_*` secrets and talks to
the API on `127.0.0.1:6443`.

The production values fit the current server: 1 vCPU, 2 GB RAM, shared with two
Docker services. **On one shared vCPU, time verdicts are not reliable.** The
sandbox shares the core with the rest of the stack, so it is a trial environment,
not one for real contests. Dedicated judge cores are #216. Disk is also tight: the
go-judge image is about 5 GB unpacked, and a release that changes the toolchain keeps
two versions until the kubelet's image garbage collection removes the old one (at 85%
disk use).

## GitHub secrets and variables

| Name | Kind | Value |
|---|---|---|
| `SSH_HOST`, `SSH_USER`, `SSH_KEY` | repository secrets | SSH access as the deploy user |
| `SSH_KNOWN_HOSTS` | `prod` variable | the server's host key as `gate-server <type> <key>` (the deploy pins it under that alias) |
| `KUBECONFIG` | `prod` secret | `~deployer/.kube/config` from the server (`server: https://127.0.0.1:6443`, reached through the tunnel). It grants cluster-admin. |
| `POSTGRES_PASSWORD`, `REDIS_PASSWORD` | `prod` secrets | random strings; `deploy/secrets.sh` escapes them in connection URLs |
| `S3_ACCESS_KEY`, `S3_SECRET_KEY` | `prod` secrets | random strings; SeaweedFS and the backend share them |
| `RESEND_API_KEY` | `prod` secret | Resend API key for email |

`deploy/secrets.sh` writes the application values to the `gate-secrets` secret. The
images in ghcr.io are public, so the cluster pulls them without credentials.

## Local cluster

`task k3d:deploy` creates a k3d cluster if needed, builds the images, and installs
both releases. Passwords come from `local/.env`. The app is then at
https://gate.localhost:9443 (self-signed certificate). `task k3d:down` deletes the
cluster. Every `kubectl`/`helm` call in these tasks names the `k3d-gate` context.
