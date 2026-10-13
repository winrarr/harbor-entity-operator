.DEFAULT_GOAL := help

## Image to build and deploy.
IMG ?= harbor-entity-operator:local
HARBOR_API_GROUP ?= harbor.harbor-entity-operator.io
CRD_REF_DOCS_OUTPUT ?= docs/reference/api.md
DOCS_CONTAINER_IMAGE ?= squidfunk/mkdocs-material:9.7.7
MKDOCS_CONFIG ?= hack/mkdocs.yml
# Linked worktrees keep Git metadata outside CURDIR and reference it by its host path.
DOCS_GIT_COMMON_DIR = $(shell git rev-parse --path-format=absolute --git-common-dir)
DOCS_CONTAINER_MOUNTS = -v "$(CURDIR)":/docs -v "$(DOCS_GIT_COMMON_DIR)":"$(DOCS_GIT_COMMON_DIR)":ro
## Kind CNI: default or cilium (with Hubble).
KIND_CNI ?= default
KIND_CLUSTER ?= kind
# renovate: datasource=docker depName=kindest/node
KIND_NODE_IMAGE ?= kindest/node:v1.37.0@sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5
KIND_CONFIG ?= $(if $(filter cilium,$(KIND_CNI)),hack/kind-configuration-cilium.yaml,hack/kind-configuration.yaml)
# renovate: datasource=docker depName=goharbor/harbor-core
HARBOR_VERSION ?= v2.15.4

# Tool binaries
LOCALBIN ?= $(CURDIR)/bin
CONTROLLER_GEN = $(LOCALBIN)/controller-gen
SETUP_ENVTEST = $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
CRD_REF_DOCS = $(LOCALBIN)/crd-ref-docs
KIND = $(LOCALBIN)/kind

# Tool versions
# renovate: datasource=github-releases depName=kubernetes-sigs/controller-tools
CONTROLLER_TOOLS_VERSION ?= v0.22.0
ENVTEST_VERSION ?= $(shell go list -m -f "{{ .Version }}" sigs.k8s.io/controller-runtime)
ENVTEST_K8S_VERSION ?= $(shell go list -m -f "{{ .Version }}" k8s.io/api | awk -F'[v.]' '{printf "1.%d", $$3}')
# renovate: datasource=github-releases depName=golangci/golangci-lint
GOLANGCI_LINT_VERSION ?= v2.14.0
# renovate: datasource=github-releases depName=elastic/crd-ref-docs
CRD_REF_DOCS_VERSION ?= v0.3.0
# renovate: datasource=github-releases depName=kubernetes-sigs/kind
KIND_VERSION ?= v0.33.0
# renovate: datasource=github-releases depName=cilium/cilium
CILIUM_VERSION ?= 1.20.2

# Use Bash and fail recipes when a command or pipeline fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

##@ General

.PHONY: help
help: ## Display available targets and variables.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m [VARIABLE=value]\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } /^## / { variable_description = substr($$0, 4); next } /^[A-Z][A-Z0-9_]*[[:space:]]*[?+:]?=/ && variable_description != "" { name = $$0; sub(/[[:space:]]*[?+:]?=.*/, "", name); variable_names[++variable_count] = name; variable_descriptions[variable_count] = variable_description; variable_description = "" } END { if (variable_count > 0) { printf "\n\033[1mCommon variables\033[0m\n"; for (i = 1; i <= variable_count; i++) printf "  \033[36m%-25s\033[0m %s\n", variable_names[i], variable_descriptions[i] } }' $(MAKEFILE_LIST)

##@ Development

.PHONY: check
check: ## Run the non-E2E CI baseline.
	$(MAKE) verify-generated
	$(MAKE) lint
	$(MAKE) test
	$(MAKE) build-docs-site

.PHONY: test
test: $(SETUP_ENVTEST) ## Run non-E2E Go tests.
	KUBEBUILDER_ASSETS="$(shell $(SETUP_ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./... -coverprofile cover.out

.PHONY: test-e2e
test-e2e: $(KIND) ## Run E2E tests against the current Kind cluster.
	@$(KIND) get clusters | grep -Fxq "$(KIND_CLUSTER)" || { \
		echo "Kind cluster '$(KIND_CLUSTER)' is not running. Start it with make kind-up."; \
		exit 1; \
	}
	KIND_CLUSTER="$(KIND_CLUSTER)" KIND_BINARY="$(KIND)" go test -tags=e2e ./test/e2e/ -v -ginkgo.v

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Check formatting and code quality.
	$(GOLANGCI_LINT) run

.PHONY: fix
fix: $(GOLANGCI_LINT) ## Apply automatic formatting and lint fixes.
	$(GOLANGCI_LINT) run --fix

##@ Generated artifacts

.PHONY: generate
generate: ## Regenerate all tracked derived artifacts.
	$(MAKE) generate-deepcopy
	$(MAKE) generate-manifests
	$(MAKE) sync-chart-assets
	$(MAKE) generate-api-reference

.PHONY: verify-generated
verify-generated: ## Verify tracked generated artifacts are current.
	./hack/check-generated-drift.sh

.PHONY: generate-deepcopy
generate-deepcopy: $(CONTROLLER_GEN) ## Generate DeepCopy implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: generate-manifests
generate-manifests: $(CONTROLLER_GEN) ## Generate CRDs and RBAC.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: sync-chart-assets
sync-chart-assets: ## Sync generated CRDs and RBAC to the chart.
	./hack/sync-chart-crds.sh
	./hack/sync-chart-rbac.sh

.PHONY: generate-api-reference
generate-api-reference: $(CRD_REF_DOCS) ## Generate the CRD API reference.
	mkdir -p $(dir $(CRD_REF_DOCS_OUTPUT))
	$(CRD_REF_DOCS) \
		--config hack/crd-ref-docs.yaml \
		--renderer markdown \
		--source-path ./api \
		--output-path $(CRD_REF_DOCS_OUTPUT)
	python3 -c 'import pathlib, sys; path = pathlib.Path(sys.argv[1]); path.write_text(path.read_text().rstrip() + "\n")' "$(CRD_REF_DOCS_OUTPUT)"

##@ Documentation

.PHONY: build-docs-site
build-docs-site: ## Build the site without regenerating its API reference.
	docker run --rm $(DOCS_CONTAINER_MOUNTS) $(DOCS_CONTAINER_IMAGE) build --strict -f $(MKDOCS_CONFIG)

.PHONY: docs-build
docs-build: ## Generate the API reference and build the site.
	$(MAKE) generate-api-reference
	$(MAKE) build-docs-site

.PHONY: docs-serve
docs-serve: generate-api-reference ## Generate and serve the site locally.
	docker run --rm -p 8000:8000 $(DOCS_CONTAINER_MOUNTS) $(DOCS_CONTAINER_IMAGE) serve --dev-addr 0.0.0.0:8000 -f $(MKDOCS_CONFIG)

.PHONY: update-harbor-openapi
update-harbor-openapi: ## Refresh the checked-in Harbor OpenAPI specification.
	python3 hack/update-harbor-openapi.py

##@ Build

.PHONY: docker-build
docker-build: ## Build the operator image.
	docker build -t $(IMG) .

##@ Deployment

.PHONY: apply-crds
apply-crds: ## Apply chart CRDs to the current cluster.
	kubectl apply -f charts/harbor-entity-operator/crds

.PHONY: prepare-deploy
prepare-deploy:
	$(MAKE) generate-deepcopy
	$(MAKE) generate-manifests
	$(MAKE) sync-chart-assets
	$(MAKE) apply-crds

.PHONY: delete-crds
delete-crds: ## Delete operator CRDs and remaining instances.
	kubectl delete --ignore-not-found -f charts/harbor-entity-operator/crds

.PHONY: deploy
deploy: prepare-deploy ## Generate assets and deploy to the current cluster.
	kubectl create namespace harbor-entity-operator-system --dry-run=client -o yaml | kubectl apply -f -
	IMG_REF="$(IMG)"; \
	if [[ "$${IMG_REF}" == *@* ]]; then echo "Digest image references are not supported by deploy"; exit 1; fi; \
	IMG_LAST="$${IMG_REF##*/}"; \
	if [[ "$${IMG_LAST}" == *:* ]]; then \
		IMG_REPO="$${IMG_REF%:*}"; \
		IMG_TAG="$${IMG_REF##*:}"; \
	else \
		IMG_REPO="$${IMG_REF}"; \
		IMG_TAG="latest"; \
	fi; \
	helm upgrade --install harbor-entity-operator ./charts/harbor-entity-operator \
		--namespace harbor-entity-operator-system \
		--set-string image.repository=$${IMG_REPO} \
		--set-string image.tag=$${IMG_TAG} \
		--wait
	kubectl rollout restart deployment/harbor-entity-operator -n harbor-entity-operator-system
	kubectl rollout status deployment/harbor-entity-operator -n harbor-entity-operator-system --timeout=180s

.PHONY: undeploy
undeploy: ## Remove the deployment; retain Harbor CRs and CRDs.
	helm uninstall harbor-entity-operator -n harbor-entity-operator-system --ignore-not-found

.PHONY: delete-harbor-crs
delete-harbor-crs: ## Delete all Harbor CRs by policy; connections last.
	HARBOR_API_GROUP=$(HARBOR_API_GROUP) hack/delete-crs.sh

##@ Samples

.PHONY: apply-samples
apply-samples: ## Apply sample Harbor CRs to the current cluster.
	kubectl apply -k config/samples

##@ Kind

.PHONY: kind-install-stack
kind-install-stack:
	helm upgrade --install traefik oci://ghcr.io/traefik/helm/traefik \
		--set service.type=NodePort \
		--set ports.web.nodePort=30080 \
		--set ports.websecure.nodePort=30443 \
		--wait
	helm repo add harbor https://helm.goharbor.io --force-update
	helm repo update harbor
	helm upgrade --install harbor harbor/harbor \
		--set nginx.image.tag=$(HARBOR_VERSION) \
		--set portal.image.tag=$(HARBOR_VERSION) \
		--set core.image.tag=$(HARBOR_VERSION) \
		--set jobservice.image.tag=$(HARBOR_VERSION) \
		--set registry.registry.image.tag=$(HARBOR_VERSION) \
		--set registry.controller.image.tag=$(HARBOR_VERSION) \
		--set trivy.image.tag=$(HARBOR_VERSION) \
		--set database.internal.image.tag=$(HARBOR_VERSION) \
		--set redis.internal.image.tag=$(HARBOR_VERSION) \
		--set exporter.image.tag=$(HARBOR_VERSION) \
		--wait
	$(MAKE) kind-refresh

.PHONY: kind-install-cilium
kind-install-cilium:
	helm repo add cilium https://helm.cilium.io --force-update
	helm repo update
	helm upgrade --install cilium cilium/cilium --version $(CILIUM_VERSION) \
		--namespace kube-system \
		--set hubble.enabled=true \
		--set hubble.relay.enabled=true \
		--set hubble.ui.enabled=true \
		--set operator.replicas=1 \
		--set k8sServiceHost=kind-control-plane \
		--set k8sServicePort=6443 \
		--wait

.PHONY: kind-up
kind-up: $(KIND) ## Create the local stack (KIND_CNI=cilium enables Hubble).
	@case "$(KIND_CNI)" in default|cilium) ;; *) echo "KIND_CNI must be 'default' or 'cilium'"; exit 1;; esac
	$(KIND) create cluster --name $(KIND_CLUSTER) --image $(KIND_NODE_IMAGE) --config $(KIND_CONFIG)
	@if [[ "$(KIND_CNI)" == "cilium" ]]; then $(MAKE) kind-install-cilium; fi
	$(MAKE) kind-install-stack

.PHONY: kind-down
kind-down: $(KIND) ## Delete the local Kind cluster.
	$(KIND) delete cluster --name $(KIND_CLUSTER)

.PHONY: kind-load-image
kind-load-image: $(KIND)
	$(KIND) load docker-image $(IMG) --name $(KIND_CLUSTER)

.PHONY: kind-refresh
kind-refresh: ## Rebuild and redeploy the operator to Kind.
	$(MAKE) docker-build
	$(MAKE) kind-load-image
	$(MAKE) deploy

.PHONY: kind-reset
kind-reset: ## Delete Harbor CRs, operator, and CRDs; retain Kind.
	$(MAKE) delete-harbor-crs
	$(MAKE) undeploy
	$(MAKE) delete-crds

.PHONY: kind-redeploy
kind-redeploy: ## Reset and redeploy the operator in Kind.
	$(MAKE) kind-reset
	$(MAKE) kind-refresh

# Tool installation
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

$(CONTROLLER_GEN): $(CONTROLLER_GEN)-$(CONTROLLER_TOOLS_VERSION)
	ln -sf $(CONTROLLER_GEN)-$(CONTROLLER_TOOLS_VERSION) $(CONTROLLER_GEN)
$(CONTROLLER_GEN)-$(CONTROLLER_TOOLS_VERSION): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

$(SETUP_ENVTEST): $(SETUP_ENVTEST)-$(ENVTEST_VERSION)
	ln -sf $(SETUP_ENVTEST)-$(ENVTEST_VERSION) $(SETUP_ENVTEST)
$(SETUP_ENVTEST)-$(ENVTEST_VERSION): $(LOCALBIN)
	$(call go-install-tool,$(SETUP_ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

$(CRD_REF_DOCS): $(CRD_REF_DOCS)-$(CRD_REF_DOCS_VERSION)
	ln -sf $(CRD_REF_DOCS)-$(CRD_REF_DOCS_VERSION) $(CRD_REF_DOCS)
$(CRD_REF_DOCS)-$(CRD_REF_DOCS_VERSION): $(LOCALBIN)
	$(call go-install-tool,$(CRD_REF_DOCS),github.com/elastic/crd-ref-docs,$(CRD_REF_DOCS_VERSION))

$(GOLANGCI_LINT): $(GOLANGCI_LINT)-$(GOLANGCI_LINT_VERSION)
	ln -sf $(GOLANGCI_LINT)-$(GOLANGCI_LINT_VERSION) $(GOLANGCI_LINT)
$(GOLANGCI_LINT)-$(GOLANGCI_LINT_VERSION): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

$(KIND): $(KIND)-$(KIND_VERSION)
	ln -sf $(KIND)-$(KIND_VERSION) $(KIND)
$(KIND)-$(KIND_VERSION): $(LOCALBIN)
	$(call go-install-tool,$(KIND),sigs.k8s.io/kind,$(KIND_VERSION))

define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef
