FROM golang:1.17-alpine3.15 AS build
COPY go.mod go.sum /work/
WORKDIR /work
RUN go mod download
COPY main.go /work/
#ENV CGO_ENABLED=0
RUN apk add --update --no-cache musl-dev gcc
RUN go build -o k3spi .

FROM rancher/k3s:v1.23.6-k3s1 as k3s

FROM alpine:3.15
RUN apk add --update --no-cache iptables openssl ca-certificates
ARG VERSION="dev"
COPY --from=build /work/k3spi /bin/k3spi
RUN mkdir -p /etc && \
    echo 'hosts: files dns' > /etc/nsswitch.conf && \
    echo "PRETTY_NAME=\"K3s ${VERSION}\"" > /etc/os-release && \
    chmod 1777 /tmp
COPY --from=k3s /bin/cni /bin/cni
COPY --from=k3s /bin/containerd /bin/
COPY --from=k3s /bin/containerd-shim-runc-v2 /bin/
COPY --from=k3s /bin/runc /bin/
RUN set -ex; \
	ln -s cni /bin/host-local; \
	ln -s cni /bin/loopback; \
	ln -s cni /bin/bridge; \
	ln -s cni /bin/portmap; \
	ln -s cni /bin/flannel; \
	ln -s k3spi /bin/kubectl; \
	ln -s k3spi /bin/crictl
VOLUME /var/lib/kubelet
VOLUME /var/lib/rancher/k3s
VOLUME /var/lib/cni
VOLUME /var/log
ENV PATH="$PATH:/bin/aux"
ENV CRI_CONFIG_FILE="/var/lib/rancher/k3s/agent/etc/crictl.yaml"
ENV K3S_KUBECONFIG_OUTPUT=/etc/kubeconfig.yaml
ENV KUBECONFIG=/etc/kubeconfig.yaml
ENTRYPOINT ["/bin/k3spi"]
CMD ["server"]
