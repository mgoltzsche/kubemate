FROM golang:1.17-alpine3.15 AS build
RUN apk add --update --no-cache musl-dev gcc
COPY go.mod go.sum /work/
WORKDIR /work
RUN go mod download
COPY main.go /work/
COPY pkg /work/pkg
#ENV CGO_ENABLED=0
RUN go build -o k3spi .

FROM rancher/k3s:v1.23.6-k3s1 AS k3s
COPY --from=build /work/k3spi /bin/k3spi

FROM alpine:3.15
RUN apk add --update --no-cache iptables openssl ca-certificates
ARG VERSION="dev"
RUN mkdir -p /etc && \
    echo 'hosts: files dns' > /etc/nsswitch.conf && \
    echo "PRETTY_NAME=\"K3s ${VERSION}\"" > /etc/os-release && \
    chmod 1777 /tmp
COPY --from=k3s /bin/cni /bin/cni
COPY --from=k3s /bin/containerd /bin/
COPY --from=k3s /bin/containerd-shim-runc-v2 /bin/
COPY --from=k3s /bin/runc /bin/
COPY --from=k3s /bin/conntrack /bin/
RUN set -ex; \
	mkdir -m2775 /output; \
	ln -s cni /bin/host-local; \
	ln -s cni /bin/loopback; \
	ln -s cni /bin/bridge; \
	ln -s cni /bin/portmap; \
	ln -s cni /bin/flannel; \
	ln -s k3spi /bin/kubectl; \
	ln -s k3spi /bin/crictl; \
	mkdir -p /etc/k3sconnect; \
	echo 'adminsecret,admin,admin,"admin,ui"' > /etc/k3sconnect/tokens
COPY --from=build /work/k3spi /bin/k3spi
COPY ./ui/dist /web
VOLUME /var/lib/kubelet
VOLUME /var/lib/rancher/k3s
VOLUME /var/lib/cni
VOLUME /var/log/pods
ENV PATH="$PATH:/bin/aux" \
	CRI_CONFIG_FILE="/var/lib/rancher/k3s/agent/etc/crictl.yaml" \
	K3S_KUBECONFIG_OUTPUT=/output/kubeconfig.yaml \
	K3S_KUBECONFIG_MODE=0640 \
	KUBECONFIG=/output/kubeconfig.yaml \
	K3SCONNECT_WEB_DIR=/web
ENTRYPOINT ["/bin/k3spi"]
CMD ["server"]
