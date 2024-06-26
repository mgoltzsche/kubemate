FROM golang:1.21-alpine3.19 AS build
RUN apk add --update --no-cache git musl-dev gcc binutils-gold
COPY go.mod go.sum /work/
WORKDIR /work
RUN go mod download
COPY main.go /work/
COPY pkg /work/pkg
ARG VERSION=dev
ENV CGO_CFLAGS=-DSQLITE_ENABLE_DBSTAT_VTAB=1
RUN set -eux; \
	VERSION_GOLANG=$(go version | cut -d" " -f3); \
	K3SPKG=github.com/k3s-io/k3s; \
	VERSIONFLAGS=" \
		-X ${K3SPKG}/pkg/version.Version=${VERSION} \
		-X ${K3SPKG}/pkg/version.UpstreamGolang=${VERSION_GOLANG} \
	"; \
	go build -o kubemate -ldflags "$VERSIONFLAGS -s -w -extldflags \"-static\"" .

FROM rancher/k3s:v1.29.3-k3s1 AS k3s
COPY --from=build /work/kubemate /bin/kubemate

FROM alpine:3.19
RUN apk add --update --no-cache iptables ip6tables ipset socat openssl ca-certificates apparmor iw wpa_supplicant dhcpcd hostapd dnsmasq
ARG VERSION="dev"
RUN set -eu; \
	ln -sf xtables-nft-multi /sbin/iptables; \
	ln -sf xtables-nft-multi /sbin/iptables-save; \
	ln -sf xtables-nft-multi /sbin/iptables-restore; \
	ln -sf xtables-nft-multi /sbin/ip6tables; \
	ln -sf xtables-nft-multi /sbin/ip6tables-save; \
	ln -sf xtables-nft-multi /sbin/ip6tables-restore; \
	mkdir -p /etc; \
    echo 'hosts: files dns' > /etc/nsswitch.conf; \
    echo "PRETTY_NAME=\"kubemate ${VERSION}\"" > /etc/os-release; \
    chmod 1777 /tmp
COPY --from=k3s /bin/cni /opt/cni/bin/cni
COPY --from=k3s /bin/containerd /bin/
COPY --from=k3s /bin/containerd-shim-runc-v2 /bin/
COPY --from=k3s /bin/runc /bin/
COPY --from=k3s /bin/conntrack /bin/
RUN set -ex; \
	mkdir -m2775 /output; \
	ln -s cni /opt/cni/bin/host-local; \
	ln -s cni /opt/cni/bin/loopback; \
	ln -s cni /opt/cni/bin/bridge; \
	ln -s cni /opt/cni/bin/portmap; \
	ln -s cni /opt/cni/bin/bandwidth; \
	ln -s cni /opt/cni/bin/flannel; \
	ln -s kubemate /bin/kubectl; \
	ln -s kubemate /bin/crictl; \
	mkdir /var/lib/dhcpd; \
	ln -s /var/lib/kubemate/dhcp/dhcpd.leases /var/lib/dhcpd/dhcpd.leases; \
	ln -s /var/lib/kubemate/rancher /etc/rancher; \
	mkdir -p /etc/kubemate; \
	echo 'adminsecret,admin,admin,"admin,ui"' > /etc/kubemate/tokens
COPY --from=build /work/kubemate /bin/kubemate
COPY dhcpcd.conf /etc/dhcpcd.conf
COPY ./config/generated/ /usr/share/kubemate/manifests/
COPY ./ui/dist/pwa /usr/share/kubemate/web
VOLUME /var/lib/kubelet
VOLUME /var/lib/kubemate
VOLUME /var/lib/cni
VOLUME /var/log/pods
VOLUME /etc/rancher
ENV PATH="$PATH:/bin/aux:/opt/cni/bin" \
	CRI_CONFIG_FILE="/var/lib/kubemate/k3s/agent/etc/crictl.yaml" \
	K3S_KUBECONFIG_OUTPUT=/output/kubeconfig.yaml \
	K3S_KUBECONFIG_MODE=0640 \
	KUBECONFIG=/output/kubeconfig.yaml \
	KUBEMATE_WEB_DIR=/usr/share/kubemate/web \
	KUBEMATE_WRITE_HOST_RESOLVCONF=false
ENTRYPOINT ["/bin/kubemate"]
CMD ["connect"]
