# Local configuration for the root Makefile.
#
# Keep supported overrides and pinned tool defaults here so the Makefile stays
# focused on workflow orchestration. Override ?= values on the command line as
# needed for local tools, images, clusters, and documentation builds.

PROJECT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/..)

## Image to build and deploy.
IMG ?= harbor-entity-operator:local
HARBOR_API_GROUP ?= harbor.harbor-entity-operator.io
CRD_REF_DOCS_OUTPUT ?= docs/reference/api.md
CONTAINER_TOOL ?= docker

## Zensical container image used to build and serve the documentation site.
# renovate: datasource=docker depName=zensical/zensical
DOCS_CONTAINER_IMAGE ?= zensical/zensical:0.0.62@sha256:162b7e191224f57b8c584debe51b157b9802efd25d3a8948e4e0f64c1baaaee6
DOCS_CONTAINER_MOUNTS = -v "$(PROJECT_DIR)":/docs
DOCS_CONFIG ?= zensical.toml

## Kind CNI: default or cilium (with Hubble).
KIND_CNI ?= default
KIND_CLUSTER ?= kind
# renovate: datasource=docker depName=kindest/node
KIND_NODE_IMAGE ?= kindest/node:v1.37.0@sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5
KIND_CONFIG ?= $(if $(filter cilium,$(KIND_CNI)),hack/kind-configuration-cilium.yaml,hack/kind-configuration.yaml)
# renovate: datasource=docker depName=goharbor/harbor-core
HARBOR_VERSION ?= v2.15.4
# renovate: datasource=github-releases depName=cilium/cilium
CILIUM_VERSION ?= 1.20.2

# Tool binaries.
LOCALBIN ?= $(PROJECT_DIR)/bin
CONTROLLER_GEN = $(LOCALBIN)/controller-gen
SETUP_ENVTEST = $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
CRD_REF_DOCS = $(LOCALBIN)/crd-ref-docs
KIND = $(LOCALBIN)/kind

# Tool versions.
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
