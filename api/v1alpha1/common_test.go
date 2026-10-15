package v1alpha1

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreationPolicyCapabilities(t *testing.T) {
	tests := []struct {
		name           string
		policy         CreationPolicy
		allowsCreate   bool
		allowsAdoption bool
	}{
		{name: "zero value defaults to create", allowsCreate: true},
		{name: "create", policy: CreationPolicyCreate, allowsCreate: true},
		{name: "adopt", policy: CreationPolicyAdopt, allowsAdoption: true},
		{name: "create or adopt", policy: CreationPolicyCreateOrAdopt, allowsCreate: true, allowsAdoption: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.policy.AllowsCreation(); got != tt.allowsCreate {
				t.Fatalf("AllowsCreation() = %t, want %t", got, tt.allowsCreate)
			}
			if got := tt.policy.AllowsAdoption(); got != tt.allowsAdoption {
				t.Fatalf("AllowsAdoption() = %t, want %t", got, tt.allowsAdoption)
			}
		})
	}
}

func TestHarborConnectionReferenceZeroValueDetection(t *testing.T) {
	if !(HarborConnectionReference{}).IsZero() {
		t.Fatal("empty connection reference should be zero")
	}
	for _, ref := range []HarborConnectionReference{
		{Name: "harbor"},
		{Kind: HarborConnectionReferenceKindCluster},
	} {
		if ref.IsZero() {
			t.Fatalf("non-empty connection reference %#v reported as zero", ref)
		}
	}
}

func TestDeletionPolicyDefaultsPreserveResourceAndSingletonSemantics(t *testing.T) {
	resource := HarborSpecBase{}
	singleton := HarborCVEAllowlistSpec{}
	explicitResource := HarborSpecBase{DeletionPolicy: DeletionPolicyOrphan}
	explicitSingleton := HarborCVEAllowlistSpec{DeletionPolicy: DeletionPolicyDelete}
	if got := resource.GetDeletionPolicy(); got != DeletionPolicyDelete {
		t.Fatalf("Harbor resource default deletion policy = %q, want Delete", got)
	}
	if got := singleton.GetDeletionPolicy(); got != DeletionPolicyOrphan {
		t.Fatalf("singleton default deletion policy = %q, want Orphan", got)
	}
	if got := explicitResource.GetDeletionPolicy(); got != DeletionPolicyOrphan {
		t.Fatalf("explicit Harbor resource deletion policy = %q, want Orphan", got)
	}
	if got := explicitSingleton.GetDeletionPolicy(); got != DeletionPolicyDelete {
		t.Fatalf("explicit singleton deletion policy = %q, want Delete", got)
	}
}

func TestDriftDetectionAccessorsPreserveConfiguredInterval(t *testing.T) {
	resource := HarborSpecBase{}
	claim := HarborClaimSpecBase{}
	allowlist := HarborCVEAllowlistSpec{}
	if resource.GetDriftDetectionInterval() != nil || claim.GetDriftDetectionInterval() != nil || allowlist.GetDriftDetectionInterval() != nil {
		t.Fatal("unset drift detection interval should remain unset")
	}
	interval := &metav1.Duration{Duration: 45 * time.Second}
	resource.DriftDetectionInterval = interval
	claim.DriftDetectionInterval = interval
	allowlist.DriftDetectionInterval = interval
	if got := resource.GetDriftDetectionInterval(); got != interval {
		t.Fatalf("Harbor spec drift interval = %p, want original %p", got, interval)
	}
	if got := claim.GetDriftDetectionInterval(); got != interval {
		t.Fatalf("claim drift interval = %p, want original %p", got, interval)
	}
	if got := allowlist.GetDriftDetectionInterval(); got != interval {
		t.Fatalf("allowlist drift interval = %p, want original %p", got, interval)
	}
}
