# Product direction

Harbor Entity Operator lets platform teams manage Harbor's control-plane resources through Kubernetes custom resources. Teams declare Harbor connections, projects, identities, registry integrations, policies, and instance settings beside the workloads that depend on them. The operator reconciles those declarations through Harbor's v2 REST API and reports progress and failures in Kubernetes status.

The operator targets an existing Harbor installation. It does not install Harbor or manage image contents such as pushed artifacts, repository tags, scans, or replication runs. Those are data-plane objects or one-shot operations rather than stable control-plane configuration.

The public API is based on Harbor's native scope and identity model. Named resources use Kubernetes names when Harbor provides a stable name; references connect dependent resources; connection and Secret references keep credentials out of custom resource specs. Finalizers, adoption rules, deletion policies, drift detection, and status make Harbor-side lifecycle behavior explicit.

The first useful product slice is the complete set of supported Harbor control-plane CRDs and reconcilers, with generated install assets, resource guides, unit and HTTP contract tests, and a Kind workflow that exercises the operator against Harbor.
