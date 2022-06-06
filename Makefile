BUILD_DIR:=$(CURDIR)/build
TOOLS_DIR:=$(BUILD_DIR)/tools

OAPI_CODEGEN_VERSION = v1.9.0
OAPI_CODEGEN = $(TOOLS_DIR)/oapi-codegen

KUBE_OPENAPI_GEN = $(TOOLS_DIR)/openapi-gen
KUBE_OPENAPI_GEN_VERSION = 5e7f5fdc6da62df0ce329920a63eda22f95b9614

CONTROLLER_GEN = $(TOOLS_DIR)/controller-gen
CONTROLLER_GEN_VERSION = v0.4.1

all: image

.PHONY: kubemate
kubemate:
	go build -o $(BUILD_DIR)/bin/kubemate .

image:
	docker build --force-rm -t kubemate .

generate: generate-types generate-openapi

generate-types: $(OAPI_CODEGEN) $(CONTROLLER_GEN) $(KUBE_OPENAPI_GEN)
	#PATH="$(TOOLS_DIR):$$PATH" go generate ./pkg/server
	$(CONTROLLER_GEN) object paths=./pkg/apis/...
	$(KUBE_OPENAPI_GEN) --output-base=./pkg/generated --output-package=openapi -O zz_generated.openapi -h ./boilerplate/boilerplate.go.txt \
		--input-dirs=github.com/mgoltzsche/kubemate/pkg/apis/devices/v1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/runtime

generate-openapi: generate-types kubemate
	@echo Load OpenAPI spec from freshly built server binary
	@{ \
	set -eu; \
	mkdir -p /tmp/kubemate-gen; \
	$(BUILD_DIR)/bin/kubemate connect --data-dir=/tmp/kubemate-gen --manifest-dir=/tmp/kubemate-gen & \
	PID=$$!; \
	sleep 1; \
	printf '# This file is generated using `make generate-openapi`.\n# DO NOT EDIT MANUALLY!\n\n' > openapi.yaml; \
	curl -fsS http://localhost:8080/openapi/v2 >> openapi.yaml; \
	kill -9 $$PID; \
	}

clean:
	[ "`id -u`" -eq  0 ]
	docker rm -f `docker ps -qa` || true
	rm -rf ./data
	docker volume rm `docker volume ls -q` || true

run: image
	chmod 2775 .
	mkdir -p ./data/pod-log
	docker run --name kubemate --rm --network host --pid host --privileged \
		--tmpfs /run --tmpfs /var/run \
		-v `pwd`/data/kubemate:/var/lib/kubemate \
		--mount type=bind,src=/etc/machine-id,dst=/etc/machine-id \
		--mount type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock \
		--mount type=bind,src=/var/lib/docker,dst=/var/lib/docker,bind-propagation=rshared \
		--mount type=bind,src=/var/lib/kubelet,dst=/var/lib/kubelet,bind-propagation=rshared \
		--mount type=bind,src=`pwd`/data/pod-log,dst=/var/log/pods,bind-propagation=rshared \
		--mount type=bind,src=/sys,dst=/sys \
		-v `pwd`:/output \
		kubemate:latest connect --docker
			#--http-port=80 --https-port=443
			#--no-deploy=servicelb,traefik,metrics-server \
			#--disable-cloud-controller \
			#--disable-helm-controller

run-other:
	docker run --name kubemate2 --rm -p 9090:8443 --privileged \
		--mount type=bind,src=/etc/machine-id,dst=/etc/machine-id \
		-v `pwd`/output:/output \
		kubemate:latest connect

$(OAPI_CODEGEN): ## Installs oapi-codegen
	$(call go-get-tool,$(OAPI_CODEGEN),github.com/deepmap/oapi-codegen/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION))

$(CONTROLLER_GEN):
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION))

$(KUBE_OPENAPI_GEN):
	$(call go-get-tool,$(KUBE_OPENAPI_GEN),k8s.io/kube-openapi/cmd/openapi-gen@$(KUBE_OPENAPI_GEN_VERSION))

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
