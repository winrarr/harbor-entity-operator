package controller

import (
	"context"
	"net/http"
	"strings"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

func TestProjectAndRegistryReferenceResolutionBoundaries(t *testing.T) {
	ctx := context.Background()
	allowCrossNamespace := false
	project := &harborv1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "team-a"}, Status: harborv1alpha1.ProjectStatus{HarborProjectID: 42}}
	registry := &harborv1alpha1.Registry{ObjectMeta: metav1.ObjectMeta{Name: "upstream", Namespace: "team-a"}, Status: harborv1alpha1.RegistryStatus{HarborRegistryID: 17}}
	unreadyProject := &harborv1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "pending", Namespace: "team-a"}}
	unreadyRegistry := &harborv1alpha1.Registry{ObjectMeta: metav1.ObjectMeta{Name: "pending", Namespace: "team-a"}}
	kubeClient := newAdvancedUnitClient(t, project, registry, unreadyProject, unreadyRegistry)
	options := OperatorOptions{allowCrossNamespaceReferences: &allowCrossNamespace}

	if name, id, err := resolveProject(ctx, options, kubeClient, "team-a", &harborv1alpha1.ProjectReference{Name: "demo"}); err != nil || name != "42" || id != 42 {
		t.Fatalf("resolve project = %q/%d, %v", name, id, err)
	}
	if name, err := resolveProjectName(ctx, options, kubeClient, "team-a", &harborv1alpha1.ProjectReference{Name: "demo"}); err != nil || name != "demo" {
		t.Fatalf("resolve project name = %q, %v", name, err)
	}
	if id, err := resolveRegistryID(ctx, options, kubeClient, "team-a", &harborv1alpha1.RegistryReference{Name: "upstream"}); err != nil || id != 17 {
		t.Fatalf("resolve registry ID = %d, %v", id, err)
	}
	if name, err := resolveUserName(ctx, options, kubeClient, "team-a", harborv1alpha1.UserReference{Name: "missing"}); err == nil || name != "" {
		t.Fatalf("missing user resolution = %q, %v; want Kubernetes not-found", name, err)
	}

	for _, test := range []struct {
		name string
		call func() error
		want string
	}{
		{name: "project reference absent", call: func() error { _, _, err := resolveProject(ctx, options, kubeClient, "team-a", nil); return err }, want: "projectRef is required"},
		{name: "project reference name absent", call: func() error {
			_, _, err := resolveProject(ctx, options, kubeClient, "team-a", &harborv1alpha1.ProjectReference{})
			return err
		}, want: "projectRef.name must not be empty"},
		{name: "project absent", call: func() error {
			_, _, err := resolveProject(ctx, options, kubeClient, "team-a", &harborv1alpha1.ProjectReference{Name: "missing"})
			return err
		}, want: "resource does not exist yet"},
		{name: "project not reconciled", call: func() error {
			_, _, err := resolveProject(ctx, options, kubeClient, "team-a", &harborv1alpha1.ProjectReference{Name: "pending"})
			return err
		}, want: "harborProjectID is not set yet"},
		{name: "registry reference absent", call: func() error { _, err := resolveRegistryID(ctx, options, kubeClient, "team-a", nil); return err }, want: "registryRef is required"},
		{name: "registry reference name absent", call: func() error {
			_, err := resolveRegistryID(ctx, options, kubeClient, "team-a", &harborv1alpha1.RegistryReference{})
			return err
		}, want: "registryRef.name must not be empty"},
		{name: "registry absent", call: func() error {
			_, err := resolveRegistryID(ctx, options, kubeClient, "team-a", &harborv1alpha1.RegistryReference{Name: "missing"})
			return err
		}, want: "resource does not exist yet"},
		{name: "registry not reconciled", call: func() error {
			_, err := resolveRegistryID(ctx, options, kubeClient, "team-a", &harborv1alpha1.RegistryReference{Name: "pending"})
			return err
		}, want: "status.harborRegistryID is not set yet"},
		{name: "cross-namespace project reference blocked", call: func() error {
			_, _, err := resolveProject(ctx, options, kubeClient, "team-a", &harborv1alpha1.ProjectReference{Name: "demo", Namespace: "team-b"})
			return err
		}, want: "cross-namespace"},
		{name: "cross-namespace registry reference blocked", call: func() error {
			_, err := resolveRegistryID(ctx, options, kubeClient, "team-a", &harborv1alpha1.RegistryReference{Name: "upstream", Namespace: "team-b"})
			return err
		}, want: "cross-namespace"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.call()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("resolution error = %v, want message containing %q", err, test.want)
			}
		})
	}

}

func TestRobotValidationReferencesAndPermissionNormalization(t *testing.T) {
	valid := &harborv1alpha1.Robot{
		ObjectMeta: metav1.ObjectMeta{Name: "release", Namespace: "team-a"},
		Spec: harborv1alpha1.RobotSpec{
			Level:     "project",
			Duration:  -1,
			SecretRef: &harborv1alpha1.SecretReference{Name: "token"},
			Permissions: []harborv1alpha1.RobotPermission{{
				Kind: "project", Access: []harborv1alpha1.RobotAccess{{Resource: "repository", Action: "push"}},
			}},
		},
	}
	if err := validateRobotSpec(valid); err != nil {
		t.Fatalf("valid Robot spec: %v", err)
	}
	for _, test := range []struct {
		name string
		edit func(*harborv1alpha1.Robot)
		want string
	}{
		{name: "missing level", edit: func(cr *harborv1alpha1.Robot) { cr.Spec.Level = "" }, want: "spec.level is required"},
		{name: "missing permissions", edit: func(cr *harborv1alpha1.Robot) { cr.Spec.Permissions = nil }, want: "permissions must contain"},
		{name: "zero duration", edit: func(cr *harborv1alpha1.Robot) { cr.Spec.Duration = 0 }, want: "duration must"},
		{name: "missing credential outputs", edit: func(cr *harborv1alpha1.Robot) { cr.Spec.SecretRef = nil }, want: "at least one of"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cr := valid.DeepCopy()
			test.edit(cr)
			if err := validateRobotSpec(cr); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("invalid Robot spec error = %v, want %q", err, test.want)
			}
		})
	}

	secretRef, err := resolveRobotSecretRef(valid)
	if err != nil || secretRef == nil || secretRef.Namespace != "team-a" || secretRef.Key != "secret" {
		t.Fatalf("default robot Secret reference = %#v, %v", secretRef, err)
	}
	imageRef, err := resolveImagePullSecretRef(valid, secretRef)
	if err != nil || imageRef != nil {
		t.Fatalf("absent image pull reference = %#v, %v", imageRef, err)
	}
	for _, test := range []struct {
		name string
		cr   *harborv1alpha1.Robot
		want string
	}{
		{name: "empty robot Secret name", cr: &harborv1alpha1.Robot{ObjectMeta: valid.ObjectMeta, Spec: harborv1alpha1.RobotSpec{SecretRef: &harborv1alpha1.SecretReference{}}}, want: "spec.secretRef.name is required"},
		{name: "reserved robot username key", cr: &harborv1alpha1.Robot{ObjectMeta: valid.ObjectMeta, Spec: harborv1alpha1.RobotSpec{SecretRef: &harborv1alpha1.SecretReference{Name: "token", Key: robotUsernameSecretKey}}}, want: "reserved"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := resolveRobotSecretRef(test.cr); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Robot Secret reference error = %v, want %q", err, test.want)
			}
		})
	}

	imagePull := valid.DeepCopy()
	imagePull.Spec.ImagePullSecretRef = &harborv1alpha1.SecretObjectReference{Name: "docker"}
	imageRef, err = resolveImagePullSecretRef(imagePull, secretRef)
	if err != nil || imageRef == nil || imageRef.Namespace != "team-a" {
		t.Fatalf("default image pull Secret reference = %#v, %v", imageRef, err)
	}
	imagePull.Spec.ImagePullSecretRef.Name = "token"
	if _, err := resolveImagePullSecretRef(imagePull, secretRef); err == nil || !strings.Contains(err.Error(), "different Secret") {
		t.Fatalf("shared token/image-pull Secret reference error = %v", err)
	}
	imagePull.Spec.ImagePullSecretRef = &harborv1alpha1.SecretObjectReference{}
	if _, err := resolveImagePullSecretRef(imagePull, secretRef); err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("empty image pull Secret reference error = %v", err)
	}

	permissions := []harborclient.RobotPermission{
		{Kind: "system", Namespace: "/", Access: []harborclient.Access{{Resource: "tag", Action: "push"}, {Resource: "artifact", Action: "pull"}}},
		{Kind: "project", Namespace: "demo", Access: []harborclient.Access{{Resource: "repository", Action: "read"}}},
	}
	permuted := []harborclient.RobotPermission{
		{Kind: "project", Namespace: "demo", Access: []harborclient.Access{{Resource: "repository", Action: "read"}}},
		{Kind: "system", Namespace: "/", Access: []harborclient.Access{{Resource: "artifact", Action: "pull"}, {Resource: "tag", Action: "push"}}},
	}
	if !robotPermissionsEqual(permissions, permuted) {
		t.Fatal("robot permission ordering caused a false drift result")
	}
	if permissions[0].Access[0].Resource != "tag" || permissions[0].Kind != "system" {
		t.Fatalf("permission normalization mutated its input: %#v", permissions)
	}
	if !intPtrEqual(nil, nil) || intPtrEqual(nil, intPointer(1)) || intPtrEqual(intPointer(1), intPointer(2)) || !intPtrEqual(intPointer(1), intPointer(1)) {
		t.Fatal("integer pointer equality did not distinguish nil and unequal values")
	}
}

func TestRetentionAndReplicationConversionRejectMalformedJSON(t *testing.T) {
	malformed := apiextensionsv1.JSON{Raw: []byte(`{`)}
	if _, err := toRetentionPolicy(harborv1alpha1.RetentionPolicy{Spec: harborv1alpha1.RetentionPolicySpec{
		Trigger: &harborv1alpha1.RetentionTrigger{Kind: "Schedule"},
		Rules:   []harborv1alpha1.RetentionRule{{Params: map[string]apiextensionsv1.JSON{"latestPushedK": malformed}}},
	}}); err == nil || !strings.Contains(err.Error(), "invalid params for rule 0") {
		t.Fatalf("invalid retention rule JSON error = %v", err)
	}
	if _, err := toRetentionTrigger(&harborv1alpha1.RetentionTrigger{Settings: map[string]apiextensionsv1.JSON{"cron": malformed}}); err == nil || !strings.Contains(err.Error(), "invalid trigger settings") {
		t.Fatalf("invalid retention trigger settings error = %v", err)
	}
	if _, err := toRetentionTrigger(&harborv1alpha1.RetentionTrigger{References: map[string]apiextensionsv1.JSON{"id": malformed}}); err == nil || !strings.Contains(err.Error(), "invalid trigger references") {
		t.Fatalf("invalid retention trigger references error = %v", err)
	}
	if trigger, err := toRetentionTrigger(nil); err != nil || trigger != nil {
		t.Fatalf("nil retention trigger conversion = %#v, %v", trigger, err)
	}
	if scope := toRetentionScope(nil); scope != nil {
		t.Fatalf("nil retention scope conversion = %#v", scope)
	}
	selectors := toRetentionScopeSelectors(map[string][]harborv1alpha1.RetentionSelector{"repository": {{Kind: "doublestar", Pattern: "**"}}})
	if len(selectors["repository"]) != 1 || selectors["repository"][0].Pattern != "**" {
		t.Fatalf("retention scope selector conversion = %#v", selectors)
	}

	if filters, err := replicationFiltersFromSpec(nil); err != nil || filters != nil {
		t.Fatalf("empty replication filters = %#v, %v", filters, err)
	}
	filters, err := replicationFiltersFromSpec([]harborv1alpha1.ReplicationFilterSpec{{Type: "name", Decoration: "matches", Value: apiextensionsv1.JSON{Raw: []byte(`"release-*"`)}}})
	if err != nil || len(filters) != 1 || filters[0].Value != "release-*" {
		t.Fatalf("replication filter conversion = %#v, %v", filters, err)
	}
	if _, err := replicationFiltersFromSpec([]harborv1alpha1.ReplicationFilterSpec{{Value: malformed}}); err == nil || !strings.Contains(err.Error(), "index 0") {
		t.Fatalf("invalid replication filter value error = %v", err)
	}
	if trigger := replicationTriggerFromSpec(nil); trigger != nil {
		t.Fatalf("nil replication trigger conversion = %#v", trigger)
	}
	trigger := replicationTriggerFromSpec(&harborv1alpha1.ReplicationTriggerSpec{Type: "scheduled", Settings: &harborv1alpha1.ReplicationTriggerSettings{Cron: "0 * * * *"}})
	if trigger == nil || trigger.Type != "scheduled" || trigger.TriggerSettings == nil || trigger.TriggerSettings.Cron != "0 * * * *" {
		t.Fatalf("scheduled replication trigger conversion = %#v", trigger)
	}
}

func TestRetentionScopeResolutionAndQuotaComparisonBoundaries(t *testing.T) {
	ctx := context.Background()
	project := &harborv1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "team-a"}, Status: harborv1alpha1.ProjectStatus{HarborProjectID: 42}}
	unready := &harborv1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "pending", Namespace: "team-a"}}
	c := newAdvancedUnitClient(t, project, unready)
	policy := &harborv1alpha1.RetentionPolicy{ObjectMeta: metav1.ObjectMeta{Name: "retention", Namespace: "team-a"}, Spec: harborv1alpha1.RetentionPolicySpec{Scope: &harborv1alpha1.RetentionScope{Level: "project", Ref: 42}}}
	if got, err := resolveRetentionScope(ctx, OperatorOptions{}, c, policy); err != nil || got != policy.Spec.Scope {
		t.Fatalf("explicit retention scope = %#v, %v", got, err)
	}
	policy.Spec.ProjectRef = &harborv1alpha1.ProjectReference{Name: "demo"}
	policy.Spec.Scope = nil
	if got, err := resolveRetentionScope(ctx, OperatorOptions{}, c, policy); err != nil || got.Level != "project" || got.Ref != 42 {
		t.Fatalf("project-derived retention scope = %#v, %v", got, err)
	}

	for _, test := range []struct {
		name string
		edit func(*harborv1alpha1.RetentionPolicy)
		want string
	}{
		{name: "empty project reference", edit: func(p *harborv1alpha1.RetentionPolicy) { p.Spec.ProjectRef = &harborv1alpha1.ProjectReference{} }, want: "projectRef.name must not be empty"},
		{name: "scope ref conflicts with project reference", edit: func(p *harborv1alpha1.RetentionPolicy) {
			p.Spec.Scope = &harborv1alpha1.RetentionScope{Level: "project", Ref: 42}
		}, want: "scope.ref must not be set"},
		{name: "scope level conflicts with project reference", edit: func(p *harborv1alpha1.RetentionPolicy) {
			p.Spec.Scope = &harborv1alpha1.RetentionScope{Level: "system"}
		}, want: `scope.level must be "project"`},
		{name: "project not found", edit: func(p *harborv1alpha1.RetentionPolicy) {
			p.Spec.ProjectRef = &harborv1alpha1.ProjectReference{Name: "missing"}
			p.Spec.Scope = nil
		}, want: "not found"},
		{name: "project not reconciled", edit: func(p *harborv1alpha1.RetentionPolicy) {
			p.Spec.ProjectRef = &harborv1alpha1.ProjectReference{Name: "pending"}
			p.Spec.Scope = nil
		}, want: "does not have harborProjectID yet"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cr := policy.DeepCopy()
			test.edit(cr)
			if _, err := resolveRetentionScope(ctx, OperatorOptions{}, c, cr); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("retention scope error = %v, want %q", err, test.want)
			}
		})
	}

	if quotaNeedsUpdate(map[string]int64{"storage": 1}, nil) != true {
		t.Fatal("missing remote quota was considered current")
	}
	current := &harborclient.Quota{Hard: map[string]int64{"storage": 1}}
	if quotaNeedsUpdate(nil, current) || quotaNeedsUpdate(map[string]int64{"storage": 1}, current) || !quotaNeedsUpdate(map[string]int64{"storage": 2}, current) {
		t.Fatal("quota hard-limit drift comparison has incorrect nil/equal/different behavior")
	}

	reconciler := &QuotaReconciler{}
	hc := newUnitHarborClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/v2.0/quotas" {
			t.Errorf("unexpected quota discovery request %s %s", r.Method, r.URL)
		}
		writeJSON(t, w, []harborclient.Quota{})
	}))
	if _, err := reconciler.findQuotaID(ctx, hc, 42); err == nil || !strings.Contains(err.Error(), "no quota found for project 42") {
		t.Fatalf("empty quota discovery error = %v", err)
	}
}

func intPointer(value int) *int { return &value }
