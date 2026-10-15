package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProjectSpec defines the desired state of Project.
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.registryRef) ? !has(self.registryRef) : has(self.registryRef) && self.registryRef == oldSelf.registryRef",message="registryRef is immutable"
type ProjectSpec struct {
	HarborSpecBase `json:",inline"`

	// CreationPolicy controls whether the operator creates or adopts the Harbor project.
	// When omitted, the operator's default creation policy is used.
	// +kubebuilder:validation:Enum=Create;Adopt;CreateOrAdopt
	// +optional
	CreationPolicy CreationPolicy `json:"creationPolicy,omitempty"`

	// Public indicates whether the project is public.
	Public bool `json:"public"`

	// Owner is an optional field for the project owner.
	// +optional
	Owner string `json:"owner,omitempty"`

	// Metadata holds additional configuration for the Harbor project.
	// +optional
	Metadata *ProjectMetadata `json:"metadata,omitempty"`

	// CVEAllowlist configures the project's CVE allowlist. Omit it to leave the
	// existing allowlist unmanaged; provide an empty items list to clear it.
	// +optional
	CVEAllowlist *CVEAllowlist `json:"cveAllowlist,omitempty"`

	// RegistryRef references the Registry to use for proxy cache projects.
	// +optional
	RegistryRef *RegistryReference `json:"registryRef,omitempty"`
}

// ProjectMetadata defines additional metadata for the project. Nil values leave
// the corresponding Harbor fields unmanaged. Pointers allow explicit empty
// values for fields where Harbor uses an empty string to clear a setting.
// Project visibility is managed through ProjectSpec.Public.
type ProjectMetadata struct {
	// EnableContentTrust blocks pulling unsigned images when enabled.
	// +kubebuilder:validation:Enum="true";"false"
	// +optional
	EnableContentTrust *string `json:"enable_content_trust,omitempty"`
	// EnableContentTrustCosign blocks pulling images without a cosign signature when enabled.
	// +kubebuilder:validation:Enum="true";"false"
	// +optional
	EnableContentTrustCosign *string `json:"enable_content_trust_cosign,omitempty"`
	// PreventVul blocks vulnerable images from running when enabled.
	// +kubebuilder:validation:Enum="true";"false"
	// +optional
	PreventVul *string `json:"prevent_vul,omitempty"`
	// Severity is the minimum severity threshold for blocking vulnerable images.
	// +kubebuilder:validation:Enum=none;low;medium;high;critical
	// +optional
	Severity *string `json:"severity,omitempty"`
	// AutoScan scans images automatically when they are pushed.
	// +kubebuilder:validation:Enum="true";"false"
	// +optional
	AutoScan *string `json:"auto_scan,omitempty"`
	// AutoSBOMGeneration generates SBOMs automatically when subject artifacts are pushed.
	// +kubebuilder:validation:Enum="true";"false"
	// +optional
	AutoSBOMGeneration *string `json:"auto_sbom_generation,omitempty"`
	// ReuseSysCVEAllowlist makes the project use the system-wide CVE allowlist.
	// +kubebuilder:validation:Enum="true";"false"
	// +optional
	ReuseSysCVEAllowlist *string `json:"reuse_sys_cve_allowlist,omitempty"`
	// RetentionID is the Harbor ID of the project's tag retention policy.
	// +optional
	RetentionID *string `json:"retention_id,omitempty"`
	// ProxySpeedKB limits proxy-cache bandwidth in kilobits per second.
	// +optional
	ProxySpeedKB *string `json:"proxy_speed_kb,omitempty"`
	// MaxUpstreamConn is the maximum number of upstream connections per artifact.
	// Use -1 for no limit.
	// +kubebuilder:validation:Pattern=`^(-1|[0-9]+)$`
	// +optional
	MaxUpstreamConn *string `json:"max_upstream_conn,omitempty"`
	// ProxyCacheLocalOnNotFound serves a cached image when it has been removed upstream.
	// +kubebuilder:validation:Enum="true";"false"
	// +optional
	ProxyCacheLocalOnNotFound *string `json:"proxy_cache_local_on_not_found,omitempty"`
	// ProxyReferrerAPI enables proxying OCI 1.1 referrer API requests upstream.
	// +kubebuilder:validation:Enum="true";"false"
	// +optional
	ProxyReferrerAPI *string `json:"proxy_referrer_api,omitempty"`
	// ProxyCacheFilterPattern limits which upstream repositories are proxied.
	// Set an explicit empty value to allow all repositories.
	// +optional
	ProxyCacheFilterPattern *string `json:"proxy_cache_filter_pattern,omitempty"`
	// ProxyCacheFilterKind selects how ProxyCacheFilterPattern is matched.
	// +kubebuilder:validation:Enum=doublestar;regex
	// +optional
	ProxyCacheFilterKind *string `json:"proxy_cache_filter_kind,omitempty"`
}

// CVEAllowlistItem defines a single CVE allowlist entry.
type CVEAllowlistItem struct {
	// CveID is the CVE identifier, such as CVE-2024-12345.
	// +kubebuilder:validation:MinLength=1
	CveID string `json:"cveID"`
}

// CVEAllowlist defines the writable project CVE allowlist configuration.
type CVEAllowlist struct {
	// ExpiresAt is the optional expiration time as seconds since the Unix epoch.
	// Omit it for a non-expiring allowlist.
	// +kubebuilder:validation:Minimum=0
	// +optional
	ExpiresAt *int64 `json:"expiresAt,omitempty"`
	// Items is the complete desired list of CVE identifiers.
	Items []CVEAllowlistItem `json:"items"`
}

// ProjectStatus defines the observed state of Project.
type ProjectStatus struct {
	HarborStatusBase `json:",inline"`

	// HarborProjectID is the ID of the project in Harbor.
	HarborProjectID int `json:"harborProjectID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=harbor
// +kubebuilder:printcolumn:name="Public",type=boolean,JSONPath=`.spec.public`
// +kubebuilder:printcolumn:name="Registry",type=string,JSONPath=`.spec.registryRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].message`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Project is the Schema for the projects API.
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectSpec   `json:"spec,omitempty"`
	Status ProjectStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProjectList contains a list of Project.
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Project `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Project{}, &ProjectList{})
}
