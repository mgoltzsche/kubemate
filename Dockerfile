FROM golang:1.19-alpine3.16 AS build
RUN apk add --update --no-cache musl-dev gcc binutils-gold
COPY go.mod go.sum /work/
WORKDIR /work
RUN go mod download
COPY main.go /work/
COPY pkg /work/pkg
ARG VERSION=dev
ENV CGO_CFLAGS=-DSQLITE_ENABLE_DBSTAT_VTAB=1
RUN go build -o kubemate -ldflags "-X main.Version=$VERSION -s -w -extldflags \"-static\"" .

FROM rancher/k3s:v1.26.0-k3s1 AS k3s
COPY --from=build /work/kubemate /bin/kubemate

FROM alpine:3.16
RUN apk add --update --no-cache iptables socat openssl ca-certificates apparmor
RUN apk add --no-cache iptables ip6tables ipset dhcp iproute2 iw wpa_supplicant hostapd
ARG VERSION="dev"
RUN set -eu; \
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
	ln -s cni /opt/cni/bin/flannel; \
	ln -s kubemate /bin/kubectl; \
	ln -s kubemate /bin/crictl; \
	mkdir -p /etc/kubemate; \
	echo 'adminsecret,admin,admin,"admin,ui"' > /etc/kubemate/tokens
COPY --from=build /work/kubemate /bin/kubemate
COPY ./config/generated/ /usr/share/kubemate/manifests/
COPY ./ui/dist/spa /usr/share/kubemate/web
VOLUME /var/lib/kubelet
VOLUME /var/lib/kubemate
VOLUME /var/lib/cni
VOLUME /var/log/pods
ENV PATH="$PATH:/bin/aux:/opt/cni/bin" \
	CRI_CONFIG_FILE="/var/lib/kubemate/k3s/agent/etc/crictl.yaml" \
	K3S_KUBECONFIG_OUTPUT=/output/kubeconfig.yaml \
	K3S_KUBECONFIG_MODE=0640 \
	KUBECONFIG=/output/kubeconfig.yaml \
	KUBEMATE_WEB_DIR=/usr/share/kubemate/web
ENTRYPOINT ["/bin/kubemate"]
CMD ["server"]
