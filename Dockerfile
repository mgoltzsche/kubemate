FROM golang:1.17-alpine3.15 AS build
RUN apk add --update --no-cache musl-dev gcc
COPY go.mod go.sum /work/
WORKDIR /work
RUN go mod download
COPY main.go /work/
COPY pkg /work/pkg
ARG VERSION=dev
ENV CGO_CFLAGS=-DSQLITE_ENABLE_DBSTAT_VTAB=1
RUN go build -o kubemate -ldflags "-X main.Version=$VERSION -s -w -extldflags \"-static\"" .

FROM golang:1.17-alpine3.15 AS cridockerd
RUN apk add --update --no-cache musl-dev gcc
RUN apk add --update --no-cache git
ARG CRI_DOCKERD_VERSION=v0.2.1
RUN git -c advice.detachedHead=false clone --branch=$CRI_DOCKERD_VERSION --depth=1 https://github.com/Mirantis/cri-dockerd.git /work
WORKDIR /work
RUN set -eux; \
	VERSION=$(echo $CRI_DOCKERD_VERSION | sed -E 's/^v//'); \
	REVISION=$(git log -1 --pretty='%h'); \
	LDFLAGS="-X version.Version=$VERSION -X version.BuildTime=$(date +%F) -X version.GitCommit=$REVISION"; \
	go build -ldflags "$LDFLAGS" -o cri-dockerd .

FROM mgoltzsche/kustomizr:1.1.2 AS manifests
COPY ./config /config
RUN mkdir /manifests && kustomize build /config/fluxcd > /manifests/fluxcd.yaml

FROM alpine:3.15 AS manifests-old
ARG FLUX_SOURCE_CTRL_VERSION=v0.25.2
ARG FLUX_KUSTOMIZE_CTRL_VERSION=v0.26.0
RUN set -eux; \
	mkdir /manifests; \
	wget -O /manifests/source-controller.yaml https://github.com/fluxcd/source-controller/releases/download/${FLUX_SOURCE_CTRL_VERSION}/source-controller.crds.yaml; \
	wget -O /manifests/source-deploy.yaml https://github.com/fluxcd/source-controller/releases/download/${FLUX_SOURCE_CTRL_VERSION}/source-controller.deployment.yaml; \
	wget -O /manifests/kustomize-controller.yaml https://github.com/fluxcd/kustomize-controller/releases/download/${FLUX_KUSTOMIZE_CTRL_VERSION}/kustomize-controller.crds.yaml; \
	wget -O /manifests/kustomize-deploy.yaml https://github.com/fluxcd/kustomize-controller/releases/download/${FLUX_KUSTOMIZE_CTRL_VERSION}/kustomize-controller.deployment.yaml; \
	printf '\n---\n%s' "$(cat /manifests/kustomize-deploy.yaml)" >> /manifests/kustomize-controller.yaml; \
	printf '\n---\n%s' "$(cat /manifests/source-deploy.yaml)" >> /manifests/source-controller.yaml; \
	rm -f /manifests/source-deploy.yaml /manifests/kustomize-deploy.yaml

FROM rancher/k3s:v1.23.6-k3s1 AS k3s
COPY --from=build /work/kubemate /bin/kubemate

FROM alpine:3.15
RUN apk add --update --no-cache iptables openssl ca-certificates apparmor
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
#COPY --from=cridockerd /work/cri-dockerd /bin/cri-dockerd
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
#COPY --from=manifests /manifests /usr/share/kubemate/manifests
COPY --from=manifests /manifests/ /usr/share/kubemate/manifests/
COPY ./ui/dist /usr/share/kubemate/web
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
