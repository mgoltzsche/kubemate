FROM golang:1.18-alpine3.15 AS build
RUN apk add --update --no-cache musl-dev gcc binutils-gold
COPY go.mod go.sum /work/
WORKDIR /work
RUN go mod download
COPY main.go /work/
COPY pkg /work/pkg
ARG VERSION=dev
ENV CGO_CFLAGS=-DSQLITE_ENABLE_DBSTAT_VTAB=1
RUN go build -o kubemate -ldflags "-X main.Version=$VERSION -s -w -extldflags \"-static\"" .

FROM golang:1.18-alpine3.15 AS cridockerd
RUN apk add --update --no-cache musl-dev gcc binutils-gold
RUN apk add --update --no-cache git
ARG CRI_DOCKERD_VERSION=v0.2.3
RUN git -c advice.detachedHead=false clone --branch=$CRI_DOCKERD_VERSION --depth=1 https://github.com/Mirantis/cri-dockerd.git /work
WORKDIR /work
RUN set -eux; \
	VERSION=$(echo $CRI_DOCKERD_VERSION | sed -E 's/^v//'); \
	REVISION=$(git log -1 --pretty='%h'); \
	LDFLAGS="-X version.Version=$VERSION -X version.BuildTime=$(date +%F) -X version.GitCommit=$REVISION -s -w -extldflags \"-static\""; \
	go build -ldflags "$LDFLAGS" -o cri-dockerd .

FROM node:18.6-alpine3.15 AS webui
COPY ui/package.json ui/yarn.lock /src/ui/
WORKDIR /src/ui
RUN yarn install
COPY openapi.yaml /src/openapi.yaml
COPY ui /src/ui
RUN yarn generate
RUN yarn build

FROM rancher/k3s:v1.24.3-k3s1 AS k3s
COPY --from=build /work/kubemate /bin/kubemate

FROM alpine:3.15
RUN apk add --update --no-cache iptables openssl ca-certificates apparmor
RUN apk add --no-cache hostapd iptables dhcp docker iproute2 iw
ARG VERSION="dev"
RUN mkdir -p /etc && \
    echo 'hosts: files dns' > /etc/nsswitch.conf && \
    echo "PRETTY_NAME=\"kubemate ${VERSION}\"" > /etc/os-release && \
    chmod 1777 /tmp
COPY --from=k3s /bin/cni /bin/cni
COPY --from=k3s /bin/containerd /bin/
COPY --from=k3s /bin/containerd-shim-runc-v2 /bin/
COPY --from=k3s /bin/runc /bin/
COPY --from=k3s /bin/conntrack /bin/
COPY --from=cridockerd /work/cri-dockerd /bin/cri-dockerd
RUN set -ex; \
	mkdir -m2775 /output; \
	ln -s cni /bin/host-local; \
	ln -s cni /bin/loopback; \
	ln -s cni /bin/bridge; \
	ln -s cni /bin/portmap; \
	ln -s cni /bin/flannel; \
	ln -s kubemate /bin/kubectl; \
	ln -s kubemate /bin/crictl; \
	mkdir -p /etc/kubemate; \
	echo 'adminsecret,admin,admin,"admin,ui"' > /etc/kubemate/tokens
COPY --from=build /work/kubemate /bin/kubemate
COPY ./config/generated/ /usr/share/kubemate/manifests/
COPY --from=webui /src/ui/dist/spa /usr/share/kubemate/web
VOLUME /var/lib/kubelet
VOLUME /var/lib/kubemate
VOLUME /var/lib/cni
VOLUME /var/log/pods
ENV PATH="$PATH:/bin/aux" \
	CRI_CONFIG_FILE="/var/lib/kubemate/k3s/agent/etc/crictl.yaml" \
	K3S_KUBECONFIG_OUTPUT=/output/kubeconfig.yaml \
	K3S_KUBECONFIG_MODE=0640 \
	KUBECONFIG=/output/kubeconfig.yaml \
	KUBEMATE_WEB_DIR=/usr/share/kubemate/web
ENTRYPOINT ["/bin/kubemate"]
CMD ["server"]
