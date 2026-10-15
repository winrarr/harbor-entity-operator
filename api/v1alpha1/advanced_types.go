package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// HarborRolePermission describes a permission granted by a custom Harbor role.
type HarborRolePermission struct {
	// Kind is the Harbor permission category.
	// +kubebuilder:validation:MinLength=1
	Kind string `json:"kind"`
	// Namespace scopes the permission to a Harbor resource namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// Access contains the resource/action/effect rules.
	// +kubebuilder:validation:MinItems=1
	Access []HarborRoleAccess `json:"access"`
}

// HarborRoleAccess is one resource/action/effect rule in a custom role.
type HarborRoleAccess struct {
	// Resource is the Harbor resource name.
	// +kubebuilder:validation:MinLength=1
	Resource string `json:"resource"`
	// Action is the Harbor action name, such as pull, push, read, or delete.
	// +kubebuilder:validation:MinLength=1
	Action string `json:"action"`
	// Effect is the permission effect.
	// +kubebuilder:validation:MinLength=1
	Effect string `json:"effect"`
}

// HarborRoleSpec defines a custom, Harbor-global role.
// +kubebuilder:validation:XValidation:rule="!has(self.harborConnectionRef) || self.harborConnectionRef.kind == 'ClusterHarborConnection'",message="cluster-scoped HarborRole requires a ClusterHarborConnection reference"
type HarborRoleSpec struct {
	HarborSpecBase `json:",inline"`

	// CreationPolicy controls whether the role is created or adopted.
	// +kubebuilder:validation:Enum=Create;Adopt;CreateOrAdopt
	// +optional
	CreationPolicy CreationPolicy `json:"creationPolicy,omitempty"`
	// Description is the role's human-readable description.
	// +optional
	Description string `json:"description,omitempty"`
	// Permissions lists the role's Harbor API permission rules.
	// +kubebuilder:validation:MinItems=1
	Permissions []HarborRolePermission `json:"permissions"`
}

// HarborRoleStatus records the Harbor identity of a custom role.
type HarborRoleStatus struct {
	HarborStatusBase `json:",inline"`
	// HarborRoleID is the Harbor role identifier.
	HarborRoleID int `json:"harborRoleID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=harbor
// +kubebuilder:printcolumn:name="HarborID",type=integer,JSONPath=`.status.harborRoleID`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].message`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// HarborRole manages a custom Harbor system role. Built-in roles cannot be managed.
type HarborRole struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              HarborRoleSpec   `json:"spec,omitempty"`
	Status            HarborRoleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type HarborRoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HarborRole `json:"items"`
}

// HarborUserGroupSpec defines a Harbor-global user group.
// +kubebuilder:validation:XValidation:rule="!has(self.harborConnectionRef) || self.harborConnectionRef.kind == 'ClusterHarborConnection'",message="cluster-scoped HarborUserGroup requires a ClusterHarborConnection reference"
// +kubebuilder:validation:XValidation:rule="self.groupType != 1 || size(self.ldapGroupDN) > 0",message="ldapGroupDN is required for LDAP groups"
// +kubebuilder:validation:XValidation:rule="self.groupType == oldSelf.groupType",message="groupType is immutable after the Harbor group is created"
type HarborUserGroupSpec struct {
	HarborSpecBase `json:",inline"`

	// CreationPolicy controls whether the group is created or adopted.
	// +kubebuilder:validation:Enum=Create;Adopt;CreateOrAdopt
	// +optional
	CreationPolicy CreationPolicy `json:"creationPolicy,omitempty"`
	// GroupType is 1 for LDAP, 2 for HTTP, and 3 for OIDC groups. Harbor does not
	// change this field through its update endpoint, so it is immutable.
	// +kubebuilder:validation:Enum=1;2;3
	GroupType int `json:"groupType"`
	// LDAPGroupDN is the LDAP distinguished name for LDAP-backed groups.
	// +optional
	LDAPGroupDN string `json:"ldapGroupDN,omitempty"`
}

// HarborUserGroupStatus records the Harbor identity of a group.
type HarborUserGroupStatus struct {
	HarborStatusBase `json:",inline"`
	// HarborUserGroupID is the Harbor group identifier.
	HarborUserGroupID int `json:"harborUserGroupID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=harbor
// +kubebuilder:printcolumn:name="HarborID",type=integer,JSONPath=`.status.harborUserGroupID`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].message`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// HarborUserGroup manages a Harbor-global user group.
type HarborUserGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              HarborUserGroupSpec   `json:"spec,omitempty"`
	Status            HarborUserGroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type HarborUserGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HarborUserGroup `json:"items"`
}

// HarborCVEAllowlistItem identifies a CVE excluded from Harbor vulnerability reports.
type HarborCVEAllowlistItem struct {
	// CVEID is the CVE identifier, such as CVE-2024-12345.
	// +kubebuilder:validation:MinLength=1
	CVEID string `json:"cveID"`
}

// HarborCVEAllowlistSpec defines Harbor's system-wide CVE allowlist.
// +kubebuilder:validation:XValidation:rule="!has(self.harborConnectionRef) || self.harborConnectionRef.kind == 'ClusterHarborConnection'",message="cluster-scoped HarborCVEAllowlist requires a ClusterHarborConnection reference"
// +kubebuilder:validation:XValidation:rule="!has(self.driftDetectionInterval) || !self.driftDetectionInterval.startsWith('-')",message="driftDetectionInterval must not be negative"
type HarborCVEAllowlistSpec struct {
	// HarborConnectionRef references the Harbor instance to configure.
	// +optional
	HarborConnectionRef *HarborConnectionReference `json:"harborConnectionRef,omitempty"`
	// DeletionPolicy controls behavior when this singleton owner is deleted.
	// Delete clears the Harbor allowlist; Orphan leaves it unchanged. Defaults to Orphan.
	// +kubebuilder:default=Orphan
	// +kubebuilder:validation:Enum=Delete;Orphan
	// +optional
	DeletionPolicy DeletionPolicy `json:"deletionPolicy,omitempty"`
	// DriftDetectionInterval controls periodic drift checks.
	// +optional
	DriftDetectionInterval *metav1.Duration `json:"driftDetectionInterval,omitempty"`
	// ReconcileNonce forces an immediate reconcile when updated.
	// +optional
	ReconcileNonce string `json:"reconcileNonce,omitempty"`
	// Items is the complete desired list of CVEs excluded from reports.
	// +optional
	Items []HarborCVEAllowlistItem `json:"items,omitempty"`
	// ExpiresAt is the optional expiration time for all entries.
	// +optional
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`
}

func (spec *HarborCVEAllowlistSpec) GetDeletionPolicy() DeletionPolicy {
	if spec.DeletionPolicy == "" {
		return DeletionPolicyOrphan
	}
	return spec.DeletionPolicy
}

func (spec *HarborCVEAllowlistSpec) GetDriftDetectionInterval() *metav1.Duration {
	return spec.DriftDetectionInterval
}

// HarborCVEAllowlistStatus records the last observed Harbor update time.
type HarborCVEAllowlistStatus struct {
	HarborStatusBase `json:",inline"`
	// HarborUpdatedAt is Harbor's update timestamp for this allowlist.
	// +optional
	HarborUpdatedAt *metav1.Time `json:"harborUpdatedAt,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=harbor
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].message`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// HarborCVEAllowlist owns Harbor's system-wide CVE allowlist singleton.
type HarborCVEAllowlist struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              HarborCVEAllowlistSpec   `json:"spec,omitempty"`
	Status            HarborCVEAllowlistStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type HarborCVEAllowlistList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HarborCVEAllowlist `json:"items"`
}

// PreheatInstanceSpec defines a Harbor-global P2P preheat provider instance.
// +kubebuilder:validation:XValidation:rule="!has(self.harborConnectionRef) || self.harborConnectionRef.kind == 'ClusterHarborConnection'",message="cluster-scoped PreheatInstance requires a ClusterHarborConnection reference"
// +kubebuilder:validation:XValidation:rule="self.authMode == 'NONE' || has(self.authInfoSecretRef)",message="authInfoSecretRef is required when authMode is not NONE"
// +kubebuilder:validation:XValidation:rule="!has(self.authInfoSecretRef) || size(self.authInfoSecretRef.namespace) > 0",message="authInfoSecretRef.namespace is required for a cluster-scoped PreheatInstance"
type PreheatInstanceSpec struct {
	HarborSpecBase `json:",inline"`

	// CreationPolicy controls whether the instance is created or adopted.
	// +kubebuilder:validation:Enum=Create;Adopt;CreateOrAdopt
	// +optional
	CreationPolicy CreationPolicy `json:"creationPolicy,omitempty"`
	// Description describes the preheat provider.
	// +optional
	Description string `json:"description,omitempty"`
	// Vendor selects the Harbor preheat driver.
	// +kubebuilder:validation:MinLength=1
	Vendor string `json:"vendor"`
	// Endpoint is the provider service endpoint.
	// It must be an absolute HTTP or HTTPS URL without query or fragment components.
	// +kubebuilder:validation:Format=uri
	// +kubebuilder:validation:Pattern=`^https?://[^/?#]+(/[^?#]*)?$`
	Endpoint string `json:"endpoint"`
	// AuthMode is the authentication mode understood by the selected provider.
	// +kubebuilder:default=NONE
	// +kubebuilder:validation:MinLength=1
	AuthMode string `json:"authMode"`
	// AuthInfoSecretRef references a Secret whose data entries are sent as provider credentials.
	// Cluster-scoped instances must set the Secret namespace explicitly.
	// +optional
	AuthInfoSecretRef *SecretObjectReference `json:"authInfoSecretRef,omitempty"`
	// Enabled controls whether Harbor uses this instance.
	Enabled bool `json:"enabled"`
	// Default marks this as Harbor's default preheat instance.
	// +optional
	Default bool `json:"default,omitempty"`
	// Insecure allows TLS connections to the provider without certificate verification.
	// +optional
	Insecure bool `json:"insecure,omitempty"`
}

// PreheatInstanceStatus records the Harbor identity of the provider instance.
type PreheatInstanceStatus struct {
	HarborStatusBase `json:",inline"`
	// HarborInstanceID is the Harbor instance identifier.
	HarborInstanceID int `json:"harborInstanceID,omitempty"`
	// HarborStatus is the latest health status reported by Harbor.
	// +optional
	HarborStatus string `json:"harborStatus,omitempty"`
	// AuthInfoHash fingerprints the referenced Secret without exposing its data.
	// +optional
	AuthInfoHash string `json:"authInfoHash,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=harbor
// +kubebuilder:printcolumn:name="Vendor",type=string,JSONPath=`.spec.vendor`
// +kubebuilder:printcolumn:name="HarborID",type=integer,JSONPath=`.status.harborInstanceID`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].message`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// PreheatInstance manages a Harbor-global preheat provider instance.
type PreheatInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              PreheatInstanceSpec   `json:"spec,omitempty"`
	Status            PreheatInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type PreheatInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PreheatInstance `json:"items"`
}

// PreheatPolicySpec defines project-level preheat behavior.
type PreheatPolicySpec struct {
	HarborSpecBase `json:",inline"`

	// CreationPolicy controls whether the policy is created or adopted.
	// +kubebuilder:validation:Enum=Create;Adopt;CreateOrAdopt
	// +optional
	CreationPolicy CreationPolicy `json:"creationPolicy,omitempty"`
	// ProjectRef identifies the Harbor Project resource this policy belongs to.
	ProjectRef ProjectReference `json:"projectRef"`
	// ProviderRef identifies the cluster-scoped PreheatInstance used by this policy.
	ProviderRef PreheatInstanceReference `json:"providerRef"`
	// Description describes the policy.
	// +optional
	Description string `json:"description,omitempty"`
	// Filters is the JSON-encoded Harbor preheat filter expression.
	// +optional
	Filters string `json:"filters,omitempty"`
	// Trigger is the JSON-encoded Harbor preheat trigger configuration.
	// +optional
	Trigger string `json:"trigger,omitempty"`
	// Enabled controls whether Harbor applies this policy.
	Enabled bool `json:"enabled"`
	// ExtraAttrs is the JSON-encoded provider-specific configuration.
	// +optional
	ExtraAttrs string `json:"extraAttrs,omitempty"`
}

// PreheatPolicyStatus records the Harbor identity of the project policy.
type PreheatPolicyStatus struct {
	HarborStatusBase `json:",inline"`
	// HarborProjectID is the Harbor project identifier used by this policy.
	HarborProjectID int `json:"harborProjectID,omitempty"`
	// HarborPreheatPolicyID is the Harbor policy identifier.
	HarborPreheatPolicyID int `json:"harborPreheatPolicyID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=harbor
// +kubebuilder:printcolumn:name="Project",type=string,JSONPath=`.spec.projectRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].message`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// PreheatPolicy manages a Harbor project preheat policy.
type PreheatPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              PreheatPolicySpec   `json:"spec,omitempty"`
	Status            PreheatPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type PreheatPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PreheatPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(
		&HarborRole{}, &HarborRoleList{},
		&HarborUserGroup{}, &HarborUserGroupList{},
		&HarborCVEAllowlist{}, &HarborCVEAllowlistList{},
		&PreheatInstance{}, &PreheatInstanceList{},
		&PreheatPolicy{}, &PreheatPolicyList{},
	)
}
