# Automated deployment recovery

The reusable deploy workflow serializes deployments per environment. It uploads
`common/rollout.py`, captures the healthy running release **before** replacing the
environment files, then copies the staged environment files into place and deploys. The VPS needs Python 3, Docker Compose v2 and curl.

`.rollback/compose.json` records each running container's immutable local image ID,
resolved environment, networks, bind paths and Traefik labels. `.rollback/files`
contains the previous environment files, including `.env` and S3 configuration.
The state directory is private (0700; generated state files 0600). Copies retain
the original file modes for bind-mounted configuration. The state must never be published
as an artifact or printed: it contains credentials. Retain it for manual recovery.
Images are deliberately not pruned, so moving registry tags cannot break rollback.

Before replacing containers, the script starts temporary backend/frontend instances
from the captured images. Temporary file-provider routes use separate router/service
names and higher priorities, retaining the previous public routes while the release changes.
A frontend using the internal backend URL is pointed at the stable backend guard
so its server-side API requests also survive the rollover.
The script probes their bridge addresses from the Docker host and waits for response
markers proving that Traefik has activated both guards before proceeding.
The usual 30-attempt, two-second health loop checks the new backend and frontend
directly, as well as the public `/health` route. A healthy guard cannot mask a broken
release. On success, the temporary routing file is removed atomically; the script
waits for healthy public responses without guard markers before removing the still-live
guard containers. This confirms Traefik has switched routes before endpoints disappear.
Handoff failure reinstates the guard routes and triggers rollback; failed recovery
retains the guard instances. Backend and
frontend Traefik services also have active healthchecks.

Pull, partial startup, or post-deploy health failures restore the captured files,
images and routing using `--pull never`, then repeat the health loop. Unchanged
databases/storage containers stay running; only replaced containers are restored.
Containers added by the failed release are removed without removing their volumes. A successful
rollback still fails the deployment job. If rollback fails, guards and recovery
state remain for the operator; logs distinguish deployment failure, rollback success
and rollback failure. On the first installation there is no prior release to restore.
A currently unhealthy deployment or unresolved guard routing is rejected before
overwriting configuration or recovery state.

Recovery never runs `down -v`, removes volumes, prunes images, or reverses database
migrations. Take the operational database/storage backups before releasing schema
changes. Migrations must leave the previous application usable; incompatible data
or schema changes require operator recovery from those backups. Traefik must watch
the host-writable directory bind-mounted at `/etc/traefik/dynamic`, as configured in
`deploy/traefik/traefik.yml`. The script discovers that mount on the shared Docker
network and writes one temporary `gate-rollover-<project>.json` file, leaving existing
configuration files intact. Activation is confirmed through public response markers;
an absent/unwatched/inaccessible directory fails before replacing release containers.
After a failed recovery, keep `.rollback` and that file together for operator recovery.
Deploy routing changes within the
existing host/path boundaries: guards preserve the old routes until activation.

Run the local failing-release exercise (isolated containers/network, no production
access) with `python3 -m unittest discover -s deploy/tests -v`. The Docker exercise
requires a running local Docker daemon and registry access for its fixture images.
