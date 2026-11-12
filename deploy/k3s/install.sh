#!/bin/sh
# Prepares a server for Gate: swap, k3s with the deploy/k3s configuration, and a
# kubeconfig for the deploy user. Run as root; safe to run again. Docker and its
# containers are left alone: k3s runs its own containerd.
#
#   sudo sh deploy/k3s/install.sh deployer
set -eu

K3S_VERSION=v1.37.1+k3s1
SWAP_SIZE=2G
DEPLOY_USER="${1:?usage: install.sh <deploy user>}"

[ "$(id -u)" = 0 ] || { echo "run as root" >&2; exit 1; }
id "$DEPLOY_USER" > /dev/null

# Swap absorbs compile and startup peaks on a small server instead of the OOM killer.
if ! swapon --show=NAME --noheadings | grep -q '^/swapfile$'; then
  [ -f /swapfile ] || { fallocate -l "$SWAP_SIZE" /swapfile; chmod 600 /swapfile; mkswap /swapfile; }
  swapon /swapfile
  grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi
echo 'vm.swappiness=10' > /etc/sysctl.d/90-gate-swap.conf
sysctl -q -p /etc/sysctl.d/90-gate-swap.conf

# metrics-server is not used; the API stays on loopback-reachable 6443 and the deploy
# reaches it through SSH.
curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION="$K3S_VERSION" sh -s - \
  --disable metrics-server --write-kubeconfig-mode 600

home="$(getent passwd "$DEPLOY_USER" | cut -d: -f6)"
install -d -m 700 -o "$DEPLOY_USER" -g "$(id -gn "$DEPLOY_USER")" "$home/.kube"
install -m 600 -o "$DEPLOY_USER" -g "$(id -gn "$DEPLOY_USER")" /etc/rancher/k3s/k3s.yaml "$home/.kube/config"

k3s kubectl wait --for=condition=Ready node --all --timeout=180s
echo "k3s $K3S_VERSION is ready; kubeconfig for $DEPLOY_USER is in $home/.kube/config"
