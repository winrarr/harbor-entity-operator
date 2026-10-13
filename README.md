# Harbor Entity Operator

`harbor-entity-operator` manages resources in an existing Harbor installation through Kubernetes custom resources. It does not install Harbor outside the repository's local development environment.

The repository uses the [Operator Foundry](https://github.com/winrarr/operator-foundry) engineering foundation and Harbor's official [v2 API specification](https://github.com/goharbor/harbor/blob/main/api/v2.0/swagger.yaml).

The operator provides namespaced `HarborConnection` and cluster-scoped `ClusterHarborConnection` resources for Harbor access. Resources for projects, users, groups, custom roles, robots, registries, policies, quotas, schedules, the system CVE allowlist, and P2P preheat reference one of those connections and reconcile desired state through Harbor's API. See the [Harbor API coverage review](docs/research/harbor-api-coverage.md) for the resource boundary and endpoint mapping.

## Run locally

Create a Kind cluster with Harbor and the operator installed:

```sh
make kind-up
```

The local workflow and live tests are described in [Local Development](docs/contributing/local-development.md).

## Install a published release

After an OCI chart release has been published, install it into a cluster that can reach Harbor:

```sh
helm registry login ghcr.io
helm upgrade --install harbor-entity-operator oci://ghcr.io/winrarr/charts/harbor-entity-operator \
  --namespace harbor-entity-operator-system \
  --create-namespace \
  --version <chart-version>
```

Create a `HarborConnection` or `ClusterHarborConnection`, then create Harbor resources that reference it. See the [quickstart](docs/quickstart.md) for the shortest complete path and the [installation guide](docs/introduction/installation.md) for chart configuration.

## Documentation

The [documentation site](https://winrarr.github.io/harbor-entity-operator/) contains:

- installation and core concepts
- resource guides and examples
- generated API reference
- connection, ownership, deletion, and multi-tenancy behavior

The Go API types and Kubebuilder markers under `api/v1alpha1` define the public Kubernetes API. The generated [API reference](docs/reference/api.md) documents that schema, while the hand-written guides explain behavior and Harbor semantics.

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for the contributor contract and [local development](docs/contributing/local-development.md) for the Kind-based workflow.
