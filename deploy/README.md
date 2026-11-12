# Deployment

Gate runs on k3s and is deployed with Helm as two releases in the `gate` namespace:

| Release | Chart | Contents | Upgrades |
|---|---|---|---|
| `gate-infra` | `charts/gate-infra` | PostgreSQL, Redis, NATS, SeaweedFS (StatefulSets on node disk) | rarely; never rolled back with the app |
| `gate` | `charts/gate` | backend, go-judge, migration Job, Traefik routes | every release |

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
- `prod/`: the production backend config (`config.yaml`, mounted with `--set-file backend.config=…`, loaded by the config tests) and chart values (domain, pull secret).
- `k3s/`: cluster bootstrap applied on every deploy. It configures the bundled Traefik (HTTPS redirect, Let's Encrypt, client IPs) and the headless `traefik-pods` service that `http.trusted_proxy` names.
- `k3d/`: the same files for the local cluster.
- `local/`: Docker Compose services for `task dev`.

## Server

A server needs:

- k3s, with the public address in the API certificate: `curl -sfL https://get.k3s.io | sh -s - --tls-san <public address>`;
- ports 80 and 443 open for users, and 6443 (the Kubernetes API) open for the deploy, which runs on GitHub-hosted runners;
- DNS for the domain in `prod/gate.yaml`;
- about 4 vCPU and 8 GB of RAM: go-judge alone is limited to 4 GiB.

## GitHub secrets (environment `prod`)

| Secret | Value |
|---|---|
| `KUBECONFIG` | kubeconfig of the cluster (`/etc/rancher/k3s/k3s.yaml` with `server:` set to the public address). It grants full cluster-admin access, so keep it only in this environment's secrets. |
| `GHCR_PULL_TOKEN` | a token with `read:packages`; the cluster pulls private images with it |
| `POSTGRES_PASSWORD`, `REDIS_PASSWORD` | random URL-safe strings (e.g. `openssl rand -hex 32`); they are put into connection URLs unescaped |
| `S3_ACCESS_KEY`, `S3_SECRET_KEY` | random strings; SeaweedFS and the backend share them |
| `RESEND_API_KEY` | Resend API key for email |

The deploy writes them to the `gate-secrets` and `ghcr` secrets in the namespace.

## Local cluster

`task k3d:deploy` creates a k3d cluster if needed, builds the images, and installs
both releases. Passwords come from `local/.env`. The app is then at
https://gate.localhost:9443 (self-signed certificate). `task k3d:down` deletes the
cluster. Every `kubectl`/`helm` call in these tasks names the `k3d-gate` context.
