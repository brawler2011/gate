# Deployment

Gate runs on k3s and is deployed with Helm as two releases in the `gate` namespace:

| Release | Chart | Contents | Upgrades |
|---|---|---|---|
| `gate-infra` | `charts/gate-infra` | PostgreSQL, NATS, SeaweedFS (StatefulSets on node disk) | rarely; never rolled back with the app |
| `gate` | `charts/gate` | API and worker Deployments, migration Job, Traefik routes | every release |

API and worker deployments scale independently through `api.replicas` and
`worker.replicas`. Both roll back together; infrastructure stays separate.

A migration hook runs before the app rollout; see
[schema reset behavior](../backend/migrations/README.md) before changing the schema. Helm restores the previous release if migration or readiness
fails. The external health check catches routing, certificate and DNS failures,
but does not trigger rollback.

Rollback restores images, not data. Run Deploy Production with an earlier release
commit to include the migration hook. Manual `helm -n gate rollback gate` skips
that hook, so use it only when the schema matches.

## Layout

- `charts/`: the two charts. Their defaults fit one node.
- `secrets.sh`: writes the `gate-secrets` secret, used by the deploy and the local tasks.
- `prod/`: the production backend config and chart values (domain, sizing).
- `k3s/`: server setup and cluster bootstrap manifests, applied on every deploy.
- `cluster/`: backend config and chart values for `task cluster:deploy`.
- `local/`: backend config, secrets and infra chart values for `task dev`.

## Server

Prepare a server once, as root:

```sh
sudo sh install.sh deployer   # deploy/k3s/install.sh copied to the server
```

It is safe to run again and leaves Docker and its containers alone. The server
needs DNS for the domain in `prod/gate.yaml`, and ports 80 and 443 open and free:
k3s's Traefik binds them.

Deploys reach the private Kubernetes API through an SSH tunnel using the `SSH_*`
secrets. Provision dedicated judge CPU capacity for reliable time verdicts, and
leave disk headroom for overlapping toolchain images during rollouts.

## Disk space

The k3s `local-path` provisioner does not enforce PVC capacity. SeaweedFS shares
the node filesystem with databases, container images and other services.

`seaweedfs.minFreeSpace` stops volume writes below the configured free-space
threshold. SeaweedFS syntax accepts an absolute amount such as `5GiB` or a
percentage such as `10`. Update it through an infra chart upgrade; app releases
do not update infrastructure.

Disk checks are periodic, and other processes can fill the filesystem, so this
is not a guaranteed reserve. Monitor free space and leave headroom for uploads
and image pulls. At the threshold, existing objects remain readable and uploads
fail with generic API errors. Free filesystem space and wait for the next disk
check to restore writes; deleting objects may not immediately shrink volume files.

## GitHub secrets and variables

| Name | Kind | Value |
|---|---|---|
| `SSH_HOST`, `SSH_USER`, `SSH_KEY` | repository secrets | SSH access as the deploy user |
| `SSH_KNOWN_HOSTS` | `prod` variable | the server's host key as `gate-server <type> <key>` (the deploy pins it under that alias) |
| `KUBECONFIG` | `prod` secret | `~deployer/.kube/config` from the server (`server: https://127.0.0.1:6443`, reached through the tunnel). It grants cluster-admin. |
| `POSTGRES_PASSWORD` | `prod` secrets | random strings; `deploy/secrets.sh` escapes them in connection URLs |
| `S3_ACCESS_KEY`, `S3_SECRET_KEY` | `prod` secrets | random strings; SeaweedFS and the backend share them |
| `RESEND_API_KEY` | `prod` secret | Resend API key for email |

The images in ghcr.io are public, so the cluster pulls them without credentials.

## Local k3s

Development machines run the same k3s, prepared once with the server script:

```sh
sudo sh deploy/k3s/install.sh "$USER"
```

It replaces `~/.kube/config` with a context named `gate`, which every local
`kubectl`/`helm` call names. Swap and AppArmor setup are skipped when the host
already has swap or runs without AppArmor. Docker keeps working beside k3s:
k3s has its own containerd, so local images reach it through
`sudo k3s ctr images import`, and the tasks below ask for sudo when they do.
NodePorts listen only on loopback. A VPN that captures all traffic must exclude
the pod and service networks (10.42.0.0/16, 10.43.0.0/16), or cluster DNS breaks.

Passwords for both environments come from `local/.env`.

- `task infra:up` installs `gate-infra` in the `gate-dev` namespace with a
  standalone go-judge that reads `backend/data/problem-work` through hostPath.
  `task dev` then runs the backend and Vite on the host against the NodePorts in
  `local/gate-infra.yaml`. `task infra:down` keeps the data; delete the
  namespace to reset it.
- `task cluster:deploy` builds the images and installs both releases in the
  `gate` namespace, as in production. The app is then at https://gate.localhost:30443
  (self-signed certificate). `task cluster:down` deletes it with its data.
