package controller

import (
	"errors"
	"strings"
	"testing"

	"github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

func TestValidatePreheatPolicyJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		spec    v1alpha1.PreheatPolicySpec
		wantErr string
	}{
		{name: "omitted JSON values"},
		{name: "valid configuration", spec: v1alpha1.PreheatPolicySpec{Filters: `[{"type":"resource","value":"artifact"}]`, Trigger: `{"type":"manual"}`, ExtraAttrs: `{}`}},
		{name: "malformed filters", spec: v1alpha1.PreheatPolicySpec{Filters: `not-json`}, wantErr: "spec.filters must be a JSON array"},
		{name: "non-array filters", spec: v1alpha1.PreheatPolicySpec{Filters: `{}`}, wantErr: "spec.filters must be a JSON array"},
		{name: "malformed trigger", spec: v1alpha1.PreheatPolicySpec{Trigger: `[`}, wantErr: "spec.trigger must be a JSON object"},
		{name: "non-object attributes", spec: v1alpha1.PreheatPolicySpec{ExtraAttrs: `[]`}, wantErr: "spec.extraAttrs must be a JSON object"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validatePreheatPolicyJSON(test.spec)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("validatePreheatPolicyJSON returned error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("validatePreheatPolicyJSON error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

func TestRedactDesiredErrorRemovesPreheatSecretValues(t *testing.T) {
	t.Parallel()

	secret := "local-e2e-token-value"
	err := redactDesiredError(errors.New("Harbor rejected auth_info local-e2e-token-value"), &harborclient.PreheatInstance{
		AuthInfo: map[string]string{"token": secret},
	})
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("redacted error still contains secret: %v", err)
	}
	if !strings.Contains(err.Error(), "[redacted]") {
		t.Fatalf("redacted error lacks replacement marker: %v", err)
	}
}
