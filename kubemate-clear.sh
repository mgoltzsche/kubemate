#!/bin/sh

[ "`id -u`" -eq  0 ] || (echo ERROR: must be run as root >&2; false) || exit 1

do_unmount_and_remove() {
    set +x
    while read -r _ path _; do
        case "$path" in $1*) echo "$path" ;; esac
    done < /proc/self/mounts | sort -r | xargs -r -t -n 1 sh -c 'umount "$0" && rm -rf "$0"'
    set -x
}

systemctl stop kubemate

"$(dirname $0)"/kill-docker-pods.sh

# Copied from https://get.k3s.io/
do_unmount_and_remove '/run/k3s'
do_unmount_and_remove '/var/lib/rancher/k3s'
do_unmount_and_remove '/var/lib/kubelet/pods'
do_unmount_and_remove '/var/lib/kubelet/plugins'
do_unmount_and_remove '/run/netns/cni-'
ip link delete cni0
ip link delete flannel.1
ip link delete flannel-v6.1
ip link delete kube-ipvs0
ip link delete flannel-wg
ip link delete flannel-wg-v6
rm -rf /var/lib/cni/
iptables-save | grep -v KUBE- | grep -v CNI- | grep -iv flannel | iptables-restore
ip6tables-save | grep -v KUBE- | grep -v CNI- | grep -iv flannel | ip6tables-restore
rm -rf /etc/rancher/k3s
rm -rf /run/k3s
rm -rf /run/flannel
rm -rf /var/lib/rancher/k3s
rm -rf /var/lib/kubelet

# Reset k3s state
# TODO: keep PersistentVolume state?!
rm -rf /var/lib/kubemate/k3s
rm -rf /var/lib/kubemate/rancher
