#!/bin/sh
# Prepares a server for Gate: swap, k3s with the API closed to the outside, and a
# kubeconfig for the deploy user. Run as root; safe to run again. Docker and its
# containers are left alone: k3s runs its own containerd.
#
#   sudo sh install.sh deployer
set -eu

K3S_VERSION=v1.37.1+k3s1
SWAP_SIZE=2G
DEPLOY_USER="${1:?usage: install.sh <deploy user>}"

[ "$(id -u)" = 0 ] || { echo "run as root" >&2; exit 1; }
id "$DEPLOY_USER" > /dev/null

# Swap takes memory peaks (compiles, startup) instead of the OOM killer. Host
# processes use it, and pods may too (LimitedSwap, below).
if ! swapon --show=NAME --noheadings | grep -qx /swapfile; then
  [ -f /swapfile ] || fallocate -l "$SWAP_SIZE" /swapfile
  chmod 600 /swapfile
  blkid -t TYPE=swap /swapfile > /dev/null || mkswap /swapfile
  swapon /swapfile
fi
grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
echo 'vm.swappiness=10' > /etc/sysctl.d/90-gate-swap.conf
sysctl -q -p /etc/sysctl.d/90-gate-swap.conf

# k3s listens for its API on every interface. The deploy reaches it through an SSH
# tunnel, so the port is closed on the interface of the default route; pods still
# reach the API through the cluster network. A unit restores the rule on boot.
cat > /usr/local/sbin/gate-close-k3s-api <<'EOF'
#!/bin/sh
set -eu
iface="$(ip route show default | awk '{print $5; exit}')"
for tables in iptables ip6tables; do
  "$tables" -C INPUT -i "$iface" -p tcp --dport 6443 -j DROP 2> /dev/null \
    || "$tables" -I INPUT -i "$iface" -p tcp --dport 6443 -j DROP
done
EOF
chmod 755 /usr/local/sbin/gate-close-k3s-api
cat > /etc/systemd/system/gate-close-k3s-api.service <<'EOF'
[Unit]
Description=Close the k3s API port to the outside
After=network-online.target
Wants=network-online.target
Before=k3s.service

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/usr/local/sbin/gate-close-k3s-api

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now gate-close-k3s-api.service

# Burstable pods may swap in proportion to their memory request. The kubelet has
# no flag for this; k3s reads drop-in kubelet configs from this directory.
install -d /var/lib/rancher/k3s/agent/etc/kubelet.conf.d
cat > /var/lib/rancher/k3s/agent/etc/kubelet.conf.d/90-gate-swap.conf <<'EOF'
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
memorySwap:
  swapBehavior: LimitedSwap
EOF

# The installer script comes from the same tag as the binary it installs.
curl -sfL "https://raw.githubusercontent.com/k3s-io/k3s/$K3S_VERSION/install.sh" \
  | INSTALL_K3S_VERSION="$K3S_VERSION" sh -s - \
    --disable metrics-server --write-kubeconfig-mode 600

home="$(getent passwd "$DEPLOY_USER" | cut -d: -f6)"
group="$(id -gn "$DEPLOY_USER")"
install -d -m 700 -o "$DEPLOY_USER" -g "$group" "$home/.kube"
install -m 600 -o "$DEPLOY_USER" -g "$group" /etc/rancher/k3s/k3s.yaml "$home/.kube/config"

# The node registers shortly after k3s starts; wait for it before waiting for Ready.
timeout 180 sh -c 'until k3s kubectl get nodes -o name 2> /dev/null | grep -q .; do sleep 2; done'
k3s kubectl wait --for=condition=Ready node --all --timeout=180s
echo "k3s $K3S_VERSION is ready; kubeconfig for $DEPLOY_USER is in $home/.kube/config"
