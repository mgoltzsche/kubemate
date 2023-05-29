VERSION ?= dev
IMAGE_REGISTRY ?= local
IMAGE = $(IMAGE_REGISTRY)/kubemate:$(VERSION)

BUILD_DIR := $(CURDIR)/build
TOOLS_DIR := $(BUILD_DIR)/tools

PACKER_IMAGE := ghcr.io/solo-io/packer-plugin-arm-image:v0.2.7

OAPI_CODEGEN_VERSION = v1.9.0
OAPI_CODEGEN = $(TOOLS_DIR)/oapi-codegen

CONTROLLER_GEN = $(TOOLS_DIR)/controller-gen
CONTROLLER_GEN_VERSION = v0.12.0

KUBE_OPENAPI_GEN = $(TOOLS_DIR)/openapi-gen
KUBE_OPENAPI_GEN_VERSION = 5e7f5fdc6da62df0ce329920a63eda22f95b9614

KUSTOMIZE := $(TOOLS_DIR)/kustomize
KUSTOMIZE_VERSION ?= v4.5.5

DOCKER ?= docker
PLATFORM ?= linux/amd64
BUILDX_OUTPUT ?= type=docker
BUILDX_BUILDER ?= kubemate-builder
BUILDX_INTERNAL_OPTS = --builder=$(BUILDX_BUILDER) --output=$(BUILDX_OUTPUT) --platform=$(PLATFORM)
BUILDX_OPTS ?=

all: container

.PHONY: kubemate
kubemate: ## Build the kubemate Go binary (without docker).
	go build -o $(BUILD_DIR)/bin/kubemate .

.PHONY: container
container: create-builder ui ## Build a linux/amd64 container image.
	mkdir -p ./build/container
	$(DOCKER) buildx build $(BUILDX_INTERNAL_OPTS) $(BUILDX_OPTS) --force-rm --build-arg VERSION=$(VERSION) -t $(IMAGE) .

.PHONY: ui
ui: ## Build a the UI.
	$(DOCKER) build --force-rm -t kubemate-webui-build:local -f Dockerfile-ui .
	$(DOCKER) rm kubemate-webui-build 2>/dev/null || true
	$(DOCKER) create --name=kubemate-webui-build kubemate-webui-build:local
	rm -rf ./ui/dist/spa
	mkdir -p ./ui/dist
	$(DOCKER) cp kubemate-webui-build:/src/ui/dist/spa ./ui/dist/spa
	$(DOCKER) rm kubemate-webui-build

.PHONY: container-multiarch
container-multiarch: PLATFORM = linux/arm64/v8,linux/amd64
container-multiarch: BUILDX_OUTPUT = type=image
container-multiarch: configure-qemu container ## Build a multi-arch container image.

.PHONY: container-tar
container-tar: PLATFORM = linux/arm64/v8
container-tar: BUILDX_OUTPUT = type=oci,dest=./build/container/kubemate-arm64.tar
container-tar: container ## Build the arm64 container OCI image tar file.

.PHONY: release
release: IMAGE_REGISTRY = docker.io/mgoltzsche
release: VERSION = latest
release: BUILDX_OPTS = --push
release: container-multiarch

.PHONY: create-builder
create-builder: ## Create the buildx builder container.
	$(DOCKER) buildx inspect $(BUILDX_BUILDER) >/dev/null 2<&1 || $(DOCKER) buildx create --name=$(BUILDX_BUILDER) >/dev/null

.PHONY: delete-builder
delete-builder: ## Delete the buildx builder container.
	$(DOCKER) buildx rm $(BUILDX_BUILDER)

.PHONY: configure-qemu
configure-qemu: ## Enable multiarch support on the host (configuring binfmt).
	$(DOCKER) run --rm --privileged multiarch/qemu-user-static:7.0.0-7 --reset -p yes

.PHONY: generate
generate: $(CONTROLLER_GEN) openapigen $(KUBE_OPENAPI_GEN) ## Generate code.
	#PATH="$(TOOLS_DIR):$$PATH" go generate ./pkg/server
	$(CONTROLLER_GEN) object paths=./pkg/apis/... paths=./pkg/resource/fake
	$(CONTROLLER_GEN) crd paths="./pkg/apis/apps/..." output:crd:artifacts:config=config/crd
	$(KUBE_OPENAPI_GEN) --output-base=./pkg/generated --output-package=openapi -O zz_generated.openapi -h ./boilerplate/boilerplate.go.txt \
		--input-dirs=github.com/mgoltzsche/kubemate/pkg/apis/devices/v1alpha1,github.com/mgoltzsche/kubemate/pkg/apis/apps/v1alpha1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/api/core/v1,k8s.io/apimachinery/pkg/runtime,k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1,k8s.io/api/networking/v1
	./build/tools/openapigen openapi.yaml

openapigen:
	go build -o ./build/tools/openapigen ./cmd/openapigen

.PHONY: manifests
manifests: $(KUSTOMIZE) ## Generate static Kubernetes manifests.
	rm -rf ./config/generated
	mkdir ./config/generated
	$(KUSTOMIZE) build ./config/fluxcd > ./config/generated/fluxcd.yaml
	$(KUSTOMIZE) build ./config/crd > ./config/generated/kubemate-crd.yaml
	$(KUSTOMIZE) build ./config/apps > ./config/generated/kubemate-apps.yaml

.PHONY: clean
clean: ## Purge local storage and docker containers created by kubemate.
	[ "`id -u`" -eq  0 ]
	docker rm -f `docker ps -qa --filter label=io.kubernetes.container.name` || true
	rm -rf ./data
	docker volume rm `docker volume ls -q --filter label=com.docker.volume.anonymous` || true
	rm -rf ./build

.PHONY: run
run: container ## Run a kubemate container locally within the host network.
	chmod 2775 .
	docker run --rm -v /:/host alpine:3.18 mkdir -p /host/var/log/pods /host/var/lib/kubelet /host/var/lib/cni /host/var/lib/kubemate
	docker rm -f kubemate 2>/dev/null || true
	docker run --name kubemate --rm -it --network host --pid host --privileged \
		--tmpfs /run --tmpfs /var/run --tmpfs /tmp \
		--mount type=bind,src=/var/lib/kubemate,dst=/var/lib/kubemate,bind-propagation=rshared \
		-v /:/host \
		--mount type=bind,src=/etc/machine-id,dst=/etc/machine-id \
		--mount type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock \
		--mount type=bind,src=/var/lib/docker,dst=/var/lib/docker,bind-propagation=rshared \
		--mount type=bind,src=/var/lib/kubelet,dst=/var/lib/kubelet,bind-propagation=rshared \
		--mount type=bind,src=/var/lib/cni,dst=/var/lib/cni \
		--mount type=bind,src=/var/log/pods,dst=/var/log/pods,bind-propagation=rshared \
		--mount type=bind,src=/lib/modules,dst=/lib/modules,readonly \
		--mount type=bind,src=/sys,dst=/sys \
		-v `pwd`:/output \
		--mount type=bind,src=`pwd`/ui/dist,dst=/usr/share/kubemate/web \
		--device /dev/snd:/dev/snd \
		$(IMAGE) connect --docker --web-dir=/usr/share/kubemate/web/spa --write-host-resolvconf --log-level=trace
			#--http-port=80 --https-port=443
			#--no-deploy=servicelb,traefik,metrics-server \
			#--disable-cloud-controller \
			#--disable-helm-controller

.PHONY: run-other
run-other: ## Run a kubemate container locally to test joining a cluster.
	docker run --name kubemate2 --rm -it -p 9090:8443 --privileged \
		--mount type=bind,src=/etc/machine-id,dst=/etc/machine-id \
		-v `pwd`/output:/output \
		--mount type=bind,src=`pwd`/ui/dist,dst=/usr/share/kubemate/web \
		$(IMAGE) connect --web-dir=/usr/share/kubemate/web/spa --log-level=trace

.PHONY: raspios-image
raspios-image: PACKER_FILE = ./packer/kubemate-raspios.pkr.hcl
raspios-image: ## Build the Raspberry Pi image.
	mkdir -p ./build
	docker run --rm --privileged \
		-v /dev:/dev \
		-v $(CURDIR):/build:ro \
		-v $(CURDIR)/build/packer_cache:/packer_cache \
		-v $(CURDIR)/output-raspios:/build/output-raspios \
		--mount type=bind,src=$(HOME)/.ssh/id_rsa.pub,dst=/root/.ssh/id_rsa.pub \
		-e PACKER_CACHE_DIR=/packer_cache \
		$(PACKER_IMAGE) \
		build $(PACKER_FILE)

.PHONY: packer-fmt
packer-fmt: ## Format the packer hcl file.
	docker run --rm -v $(CURDIR):/build \
		ghcr.io/solo-io/packer-plugin-arm-image:v0.2.6 \
		fmt ./packer/*.hcl

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

$(OAPI_CODEGEN): ## Installs oapi-codegen
	$(call go-get-tool,$(OAPI_CODEGEN),github.com/deepmap/oapi-codegen/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION))

$(CONTROLLER_GEN):
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION))

$(KUBE_OPENAPI_GEN):
	$(call go-get-tool,$(KUBE_OPENAPI_GEN),k8s.io/kube-openapi/cmd/openapi-gen@$(KUBE_OPENAPI_GEN_VERSION))

$(KUSTOMIZE):
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v4@$(KUSTOMIZE_VERSION))

# go-get-tool will 'go get' any package $2 and install it to $1.
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(TOOLS_DIR) go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef
