#!/bin/sh
set -eu

K3S_VERSION=v1.37.1+k3s1
SWAP_SIZE=2G
DEPLOY_USER="${1:?usage: install.sh <deploy user>}"

[ "$(id -u)" = 0 ] || { echo "run as root" >&2; exit 1; }
id "$DEPLOY_USER" > /dev/null

# Allow compile and startup memory peaks to spill to swap instead of triggering OOM kills.
if ! swapon --show=NAME --noheadings | grep -qx /swapfile; then
  [ -f /swapfile ] || fallocate -l "$SWAP_SIZE" /swapfile
  chmod 600 /swapfile
  blkid -t TYPE=swap /swapfile > /dev/null || mkswap /swapfile
  swapon /swapfile
fi
grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
echo 'vm.swappiness=10' > /etc/sysctl.d/90-gate-swap.conf
sysctl -q -p /etc/sysctl.d/90-gate-swap.conf

# Block the public API interface while retaining cluster access and the deploy SSH tunnel.
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

cat > /etc/apparmor.d/gate-default <<'EOF'
#include <tunables/global>

# Stacked host labels require both peer forms; containerd's profile otherwise denies container signals.
profile gate-default flags=(attach_disconnected,mediate_deleted) {
  #include <abstractions/base>

  network,
  capability,
  file,
  umount,

  signal (receive) peer=unconfined,
  signal (receive) peer=runc,
  signal (receive) peer=crun,
  signal (send,receive) peer=gate-default,
  signal (send,receive) peer=gate-default//&unconfined,
  ptrace (trace,read,tracedby,readby) peer=gate-default,
  ptrace (trace,read,tracedby,readby) peer=gate-default//&unconfined,

  deny @{PROC}/* w,
  deny @{PROC}/{[^1-9],[^1-9][^0-9],[^1-9s][^0-9y][^0-9s],[^1-9][^0-9][^0-9][^0-9/]*}/** w,
  deny @{PROC}/sys/[^k]** w,
  deny @{PROC}/sys/kernel/{?,??,[^s][^h][^m]**} w,
  deny @{PROC}/sysrq-trigger rwklx,
  deny @{PROC}/kcore rwklx,
  deny mount,
  deny /sys/[^f]*/** wklx,
  deny /sys/f[^s]*/** wklx,
  deny /sys/fs/[^c]*/** wklx,
  deny /sys/fs/c[^g]*/** wklx,
  deny /sys/fs/cg[^r]*/** wklx,
  deny /sys/firmware/** rwklx,
  deny /sys/devices/virtual/powercap/** rwklx,
  deny /sys/kernel/security/** rwklx,
}
EOF
apparmor_parser -r -W /etc/apparmor.d/gate-default

# k3s requires a kubelet config drop-in for LimitedSwap.
install -d /var/lib/rancher/k3s/agent/etc/kubelet.conf.d
cat > /var/lib/rancher/k3s/agent/etc/kubelet.conf.d/90-gate-swap.conf <<'EOF'
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
memorySwap:
  swapBehavior: LimitedSwap
EOF

curl -sfL "https://raw.githubusercontent.com/k3s-io/k3s/$K3S_VERSION/install.sh" \
  | INSTALL_K3S_VERSION="$K3S_VERSION" sh -s - \
    --disable metrics-server --write-kubeconfig-mode 600

home="$(getent passwd "$DEPLOY_USER" | cut -d: -f6)"
group="$(id -gn "$DEPLOY_USER")"
install -d -m 700 -o "$DEPLOY_USER" -g "$group" "$home/.kube"
install -m 600 -o "$DEPLOY_USER" -g "$group" /etc/rancher/k3s/k3s.yaml "$home/.kube/config"

# Wait for node registration before waiting for readiness.
timeout 180 sh -c 'until k3s kubectl get nodes -o name 2> /dev/null | grep -q .; do sleep 2; done'
k3s kubectl wait --for=condition=Ready node --all --timeout=180s
echo "k3s $K3S_VERSION is ready; kubeconfig for $DEPLOY_USER is in $home/.kube/config"
