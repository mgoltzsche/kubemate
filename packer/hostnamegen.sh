#!/bin/sh

set -eu

: ${HOST_PREFIX:=kubemate}
: ${NET_DEVICE:=eth0}
: ${HOSTNAME_FILE:=/etc/hostname}

if [ -f "$HOSTNAME_FILE" ]; then
	echo "Skipping hostname generation since $HOSTNAME_FILE already exists"
	exit 0
fi

echo "Generating new hostname since $HOSTNAME_FILE does not exist"

LAST_MAC4="$(sed -rn "s/^.*([0-9A-F:]{5})$/\1/gi;s/://p" /sys/class/net/${NET_DEVICE}/address)"
NEW_HOSTNAME="${HOST_PREFIX}-${LAST_MAC4:-0000}"

echo $NEW_HOSTNAME > "$HOSTNAME_FILE"
/bin/hostname -F "$HOSTNAME_FILE"
sync
