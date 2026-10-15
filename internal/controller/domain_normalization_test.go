package controller

import (
	"context"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

func TestLabelComparisonIgnoresHarborMetadataButDetectsDesiredDrift(t *testing.T) {
	desired := harborclient.Label{Name: "release", Description: "stable releases", Color: "#198754", Scope: "p", ProjectID: 42}
	current := &harborclient.Label{ID: 9, Name: "release", Description: "stable releases", Color: "#198754", Scope: "p", ProjectID: 42, CreationTime: "created", UpdateTime: "updated"}
	if labelNeedsUpdate(desired, current) {
		t.Fatal("Harbor-assigned ID and timestamps caused false drift")
	}
	current.Color = "#dc3545"
	if !labelNeedsUpdate(desired, current) {
		t.Fatal("label color drift was ignored")
	}
	if !labelNeedsUpdate(desired, nil) {
		t.Fatal("missing Harbor label should require creation")
	}
}

func TestImmutableRuleNormalizationAndJSONConversion(t *testing.T) {
	first := harborclient.ImmutableRule{
		ID: 10, Priority: 2, Action: "immutable", Template: "always",
		Params: map[string]any{"nested": map[string]any{"enabled": true, "count": float64(3)}},
		TagSelectors: []harborclient.ImmutableSelector{
			{Kind: "doublestar", Pattern: "v*"}, {Kind: "doublestar", Pattern: "release-*"},
		},
		ScopeSelectors: map[string][]harborclient.ImmutableSelector{
			"repository": {{Kind: "doublestar", Pattern: "prod/*"}, {Kind: "doublestar", Pattern: "dev/*"}},
		},
	}
	second := harborclient.ImmutableRule{
		ID: 99, Priority: 2, Action: "immutable", Template: "always",
		Params: map[string]any{"nested": map[string]any{"count": 3, "enabled": true}},
		TagSelectors: []harborclient.ImmutableSelector{
			{Kind: "doublestar", Pattern: "release-*"}, {Kind: "doublestar", Pattern: "v*"},
		},
		ScopeSelectors: map[string][]harborclient.ImmutableSelector{
			"repository": {{Kind: "doublestar", Pattern: "dev/*"}, {Kind: "doublestar", Pattern: "prod/*"}},
		},
	}
	if !immutableRuleMatches(first, second) || immutableRuleNeedsUpdate(first, &second) {
		t.Fatal("server IDs, JSON number representation, or selector order caused false immutable-rule drift")
	}
	if first.TagSelectors[0].Pattern != "v*" || first.ScopeSelectors["repository"][0].Pattern != "prod/*" {
		t.Fatalf("immutable comparison reordered desired selectors: %#v", first)
	}
	second.Disabled = true
	if !immutableRuleNeedsUpdate(first, &second) {
		t.Fatal("immutable-rule behavior drift was ignored")
	}
	if !immutableRuleNeedsUpdate(first, nil) {
		t.Fatal("missing Harbor rule should require creation")
	}

	converted, err := jsonMapToAnyImmutable(map[string]apiextensionsv1.JSON{
		"retention": {Raw: []byte(`{"days":7}`)},
		"omitted":   {},
	})
	if err != nil || !reflect.DeepEqual(converted, map[string]any{"retention": map[string]any{"days": float64(7)}}) {
		t.Fatalf("JSON params conversion = %#v, %v", converted, err)
	}
	if _, err := jsonMapToAnyImmutable(map[string]apiextensionsv1.JSON{"broken": {Raw: []byte(`{`)}}); err == nil || !strings.Contains(err.Error(), "broken") {
		t.Fatalf("invalid immutable-rule params error = %v, want key context", err)
	}
	if selectors := toImmutableScopeSelectors(nil); selectors != nil {
		t.Fatalf("nil scope selectors converted to %#v", selectors)
	}
}

func TestReplicationPolicyConversionAndDriftComparison(t *testing.T) {
	trigger := replicationTriggerFromSpec(&harborv1alpha1.ReplicationTriggerSpec{Type: "scheduled", Settings: &harborv1alpha1.ReplicationTriggerSettings{Cron: "0 2 * * *"}})
	if trigger == nil || trigger.Type != "scheduled" || trigger.TriggerSettings == nil || trigger.TriggerSettings.Cron != "0 2 * * *" {
		t.Fatalf("converted trigger = %#v", trigger)
	}
	if replicationTriggerFromSpec(nil) != nil {
		t.Fatal("nil trigger should remain nil")
	}
	filters, err := replicationFiltersFromSpec([]harborv1alpha1.ReplicationFilterSpec{
		{Type: "name", Value: apiextensionsv1.JSON{Raw: []byte(`{"include":["a","b"]}`)}, Decoration: "matches"},
		{Type: "tag", Value: apiextensionsv1.JSON{Raw: []byte(`true`)}},
	})
	if err != nil || len(filters) != 2 || filters[0].Value.(map[string]any)["include"] == nil || filters[1].Value != true {
		t.Fatalf("converted replication filters = %#v, %v", filters, err)
	}
	if empty, err := replicationFiltersFromSpec(nil); err != nil || empty != nil {
		t.Fatalf("empty filter conversion = %#v, %v; want nil, nil", empty, err)
	}
	if _, err := replicationFiltersFromSpec([]harborv1alpha1.ReplicationFilterSpec{{Type: "name", Value: apiextensionsv1.JSON{Raw: []byte(`{`)}}}); err == nil || !strings.Contains(err.Error(), "index 0") {
		t.Fatalf("invalid replication filter error = %v, want filter index", err)
	}

	count := 3
	desired := harborclient.ReplicationPolicy{
		ID: 1, Name: "mirror", SrcRegistry: &harborclient.Registry{ID: 10}, DestRegistry: &harborclient.Registry{ID: 20},
		DestNamespaceReplaceCount: &count,
		Filters: []harborclient.ReplicationFilter{
			{Type: "name", Decoration: "matches", Value: map[string]any{"include": []any{"a", "b"}}},
			{Type: "tag", Value: true},
		},
	}
	current := harborclient.ReplicationPolicy{
		ID: 99, Name: "mirror", SrcRegistry: &harborclient.Registry{ID: 10, Name: "source"}, DestRegistry: &harborclient.Registry{ID: 20, Name: "destination"},
		DestNamespaceReplaceCount: &count, CreationTime: "created", UpdateTime: "updated",
		Filters: []harborclient.ReplicationFilter{
			{Type: "tag", Value: true},
			{Type: "name", Decoration: "matches", Value: map[string]any{"include": []any{"a", "b"}}},
		},
	}
	if replicationPolicyNeedsUpdate(desired, &current) {
		t.Fatal("Harbor metadata, registry display fields, or filter order caused false policy drift")
	}
	if desired.Filters[0].Type != "name" {
		t.Fatalf("replication comparison reordered desired filters: %#v", desired.Filters)
	}
	current.DestRegistry.ID++
	if !replicationPolicyNeedsUpdate(desired, &current) {
		t.Fatal("destination registry drift was ignored")
	}
	if !replicationPolicyNeedsUpdate(desired, nil) {
		t.Fatal("missing replication policy should require creation")
	}
}

func TestRetentionNormalizationAndHarborConflictRecognition(t *testing.T) {
	desired := harborclient.RetentionPolicy{
		ID: 4,
		Rules: []harborclient.RetentionRule{{
			ID: 7, Priority: 1, Action: "retain", Template: "latestPushedK",
			Params: map[string]any{"latest": map[string]any{"value": float64(5)}, "include": []any{"a", "b"}},
		}},
	}
	current := harborclient.RetentionPolicy{
		ID: 99,
		Rules: []harborclient.RetentionRule{{
			ID: 88, Priority: 1, Action: "retain", Template: "latestPushedK",
			Params: map[string]any{"latest": float64(5), "include": []any{"a", "b"}},
		}},
	}
	if retentionNeedsUpdate(desired, &current) {
		t.Fatal("Harbor-assigned IDs or wrapped parameter values caused false retention drift")
	}
	if desired.Rules[0].ID != 7 {
		t.Fatalf("retention comparison mutated desired rule ID: %#v", desired.Rules[0])
	}
	current.Rules[0].Disabled = true
	if !retentionNeedsUpdate(desired, &current) || !retentionNeedsUpdate(desired, nil) {
		t.Fatal("retention rule or missing-policy drift was ignored")
	}

	for _, test := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "Harbor already bound", err: &harborclient.HTTPError{StatusCode: 400, Message: "project already has retention policy"}, want: true},
		{name: "wrong status", err: &harborclient.HTTPError{StatusCode: 409, Message: "already has retention policy"}},
		{name: "wrong message", err: &harborclient.HTTPError{StatusCode: 400, Message: "invalid request"}},
		{name: "other error", err: context.DeadlineExceeded},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := isRetentionAlreadyBound(test.err); got != test.want {
				t.Fatalf("isRetentionAlreadyBound(%v) = %t, want %t", test.err, got, test.want)
			}
		})
	}

	params, err := jsonMapToAny(map[string]apiextensionsv1.JSON{"limit": {Raw: []byte(`10`)}, "omitted": {}})
	if err != nil || params["limit"] != float64(10) || len(params) != 1 {
		t.Fatalf("retention params conversion = %#v, %v", params, err)
	}
	if _, err := jsonMapToAny(map[string]apiextensionsv1.JSON{"broken": {Raw: []byte(`no-json`)}}); err == nil {
		t.Fatal("invalid retention JSON was accepted")
	}
	if normalized := normalizeRetentionParamsAny("unchanged"); normalized != "unchanged" {
		t.Fatalf("non-map retention params = %#v, want unchanged", normalized)
	}
}

func TestWebhookTargetsResolveAndHashReferencedCredentials(t *testing.T) {
	ctx := context.Background()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "webhook-auth", Namespace: testNamespace},
		Data:       map[string][]byte{"header": []byte("first-token")},
	}
	kubeClient := newAdvancedUnitClient(t, secret)
	reconciler := &WebhookPolicyReconciler{Client: kubeClient}
	policy := &harborv1alpha1.WebhookPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "notifications", Namespace: testNamespace},
		Spec: harborv1alpha1.WebhookPolicySpec{Targets: []harborv1alpha1.WebhookTargetSpec{{
			Type: "http", Address: "https://hooks.example.test/events",
			AuthHeaderSecretRef: &harborv1alpha1.SecretReference{Name: secret.Name, Key: "header"},
		}}},
	}
	targets, firstHash, err := reconciler.buildTargets(ctx, policy)
	if err != nil || len(targets) != 1 || targets[0].AuthHeader != "first-token" || firstHash == "" {
		t.Fatalf("resolved webhook target/hash = %#v/%q, %v", targets, firstHash, err)
	}
	storedSecret := &corev1.Secret{}
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: secret.Name}, storedSecret); err != nil {
		t.Fatalf("get webhook Secret: %v", err)
	}
	storedSecret.Data["header"] = []byte("second-token")
	if err := kubeClient.Update(ctx, storedSecret); err != nil {
		t.Fatalf("rotate webhook Secret: %v", err)
	}
	rotatedTargets, rotatedHash, err := reconciler.buildTargets(ctx, policy)
	if err != nil || rotatedTargets[0].AuthHeader != "second-token" || rotatedHash == firstHash {
		t.Fatalf("rotated webhook target/hash = %#v/%q, %v; want changed credential hash", rotatedTargets, rotatedHash, err)
	}
	policy.Spec.Targets = nil
	if _, _, err := reconciler.buildTargets(ctx, policy); err == nil || !strings.Contains(err.Error(), "must not be empty") {
		t.Fatalf("empty webhook targets error = %v", err)
	}
	policy.Spec.Targets = []harborv1alpha1.WebhookTargetSpec{{
		Type: "http", Address: "https://hooks.example.test/events", AuthHeader: "literal",
		AuthHeaderSecretRef: &harborv1alpha1.SecretReference{Name: secret.Name, Key: "header"},
	}}
	if _, _, err := reconciler.buildTargets(ctx, policy); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("literal and Secret auth-header error = %v", err)
	}
}

func TestWebhookComparisonIgnoresServerFieldsAndDetectsPolicyDrift(t *testing.T) {
	desired := harborclient.WebhookPolicy{
		Name: "notify", ProjectID: 7, Description: "deliver events", Enabled: true,
		EventTypes: []string{"PUSH_ARTIFACT", "DELETE_ARTIFACT"},
		Targets: []harborclient.WebhookTarget{
			{Type: "http", Address: "https://b.example.test", AuthHeader: "new-secret", PayloadFormat: "CloudEvents"},
			{Type: "http", Address: "https://a.example.test", AuthHeader: "new-secret", PayloadFormat: "CloudEvents"},
		},
	}
	current := harborclient.WebhookPolicy{
		ID: 10, Name: "notify", ProjectID: 7, Description: "deliver events", Enabled: true,
		Creator: "admin", CreationTime: "created", UpdateTime: "updated",
		EventTypes: []string{"DELETE_ARTIFACT", "PUSH_ARTIFACT"},
		Targets: []harborclient.WebhookTarget{
			{Type: "http", Address: "https://a.example.test", AuthHeader: "old-secret", PayloadFormat: "CloudEvents"},
			{Type: "http", Address: "https://b.example.test", AuthHeader: "old-secret", PayloadFormat: "CloudEvents"},
		},
	}
	if webhookPolicyNeedsUpdate(desired, &current) {
		t.Fatal("server metadata, target order, event order, or secret value caused false webhook policy drift")
	}
	for i, target := range desired.Targets {
		if target.AuthHeader != "new-secret" {
			t.Fatalf("comparison mutated desired webhook target %d credential to %q", i, target.AuthHeader)
		}
	}
	if desired.EventTypes[0] != "PUSH_ARTIFACT" || desired.Targets[0].Address != "https://b.example.test" {
		t.Fatalf("webhook comparison reordered desired fields: events=%v targets=%#v", desired.EventTypes, desired.Targets)
	}
	current.Enabled = false
	if !webhookPolicyNeedsUpdate(desired, &current) || !webhookPolicyNeedsUpdate(desired, nil) {
		t.Fatal("enabled-state or missing-policy drift was ignored")
	}
}

func TestScanAllScheduleComparisonUsesScheduleAndParameters(t *testing.T) {
	desired := &harborclient.Schedule{Schedule: harborclient.ScheduleObj{Type: "Daily", Cron: "0 3 * * *"}, Parameters: map[string]any{"workers": float64(4), "options": map[string]any{"scan": true}}}
	current := &harborclient.Schedule{Schedule: harborclient.ScheduleObj{Type: "Daily", Cron: "0 3 * * *"}, Parameters: map[string]any{"options": map[string]any{"scan": true}, "workers": float64(4)}}
	if !scanAllSchedulesEqual(current, desired) {
		t.Fatal("map key ordering caused false scan schedule drift")
	}
	if scanAllSchedulesEqual(nil, desired) || scanAllSchedulesEqual(current, nil) {
		t.Fatal("nil Harbor or desired schedule was considered equal")
	}
	changed := *current
	changed.Schedule.Cron = "0 4 * * *"
	if scanAllSchedulesEqual(&changed, desired) {
		t.Fatal("cron drift was ignored")
	}
	changed = *current
	changed.Parameters = map[string]any{"workers": float64(8)}
	if scanAllSchedulesEqual(&changed, desired) {
		t.Fatal("scan parameter drift was ignored")
	}
}

func TestRegistryCredentialResolutionAndChangeDetection(t *testing.T) {
	ctx := context.Background()
	keySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "registry-key", Namespace: testNamespace},
		Data:       map[string][]byte{"access_key": []byte("robot-user")},
	}
	secretSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "registry-secret", Namespace: testNamespace},
		Data:       map[string][]byte{"access_secret": []byte("robot-token")},
	}
	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "registry-ca", Namespace: testNamespace},
		Data:       map[string][]byte{"ca.crt": []byte("pem-certificate")},
	}
	kubeClient := newAdvancedUnitClient(t, keySecret, secretSecret, caSecret)
	reconciler := &RegistryReconciler{Client: kubeClient}
	registry := &harborv1alpha1.Registry{ObjectMeta: metav1.ObjectMeta{Name: "upstream", Namespace: testNamespace}}
	cred := &harborv1alpha1.RegistryCredentialSpec{
		AccessKeySecretRef:    harborv1alpha1.SecretReference{Name: keySecret.Name, Namespace: testNamespace},
		AccessSecretSecretRef: harborv1alpha1.SecretReference{Name: secretSecret.Name, Namespace: testNamespace},
	}
	registry.Spec.Credential = cred
	registry.Spec.CACertificateRef = &harborv1alpha1.SecretReference{Name: caSecret.Name, Namespace: testNamespace}

	resolved, firstHash, caCert, err := reconciler.buildRegistryCredential(ctx, registry)
	if err != nil || resolved == nil || resolved.Type != "basic" || resolved.AccessKey != "robot-user" || resolved.AccessSecret != "robot-token" || caCert != "pem-certificate" || firstHash == "" {
		t.Fatalf("resolved registry credential/hash/CA = %#v/%q/%q, %v", resolved, firstHash, caCert, err)
	}
	if strings.Contains(firstHash, "robot-token") || strings.Contains(firstHash, "robot-user") {
		t.Fatalf("credential hash contains raw credential material: %q", firstHash)
	}

	storedSecret := &corev1.Secret{}
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: secretSecret.Name}, storedSecret); err != nil {
		t.Fatalf("get registry credential Secret: %v", err)
	}
	storedSecret.Data["access_secret"] = []byte("rotated-token")
	if err := kubeClient.Update(ctx, storedSecret); err != nil {
		t.Fatalf("rotate registry Secret: %v", err)
	}
	_, rotatedHash, _, err := reconciler.buildRegistryCredential(ctx, registry)
	if err != nil || rotatedHash == firstHash {
		t.Fatalf("credential hash after Secret rotation = %q, %v; want different hash", rotatedHash, err)
	}

	registry.Spec.CACertificate = "inline-pem"
	if _, _, _, err := reconciler.buildRegistryCredential(ctx, registry); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("inline and Secret CA conflict error = %v", err)
	}
	registry.Spec.CACertificate = ""
	registry.Spec.CACertificateRef = nil
	registry.Spec.Credential = &harborv1alpha1.RegistryCredentialSpec{
		AccessSecretSecretRef: harborv1alpha1.SecretReference{Name: secretSecret.Name, Namespace: testNamespace},
	}
	if _, _, _, err := reconciler.buildRegistryCredential(ctx, registry); err == nil || !strings.Contains(err.Error(), "accessKeySecretRef.name") {
		t.Fatalf("missing access-key Secret reference error = %v", err)
	}
	registry.Spec.Credential = nil
	registry.Spec.CACertificate = ""
	plain, plainHash, plainCA, err := reconciler.buildRegistryCredential(ctx, registry)
	if err != nil || plain != nil || plainHash != "" || plainCA != "" {
		t.Fatalf("registry without optional credentials = %#v/%q/%q, %v", plain, plainHash, plainCA, err)
	}
}

func TestRegistryRequestAndDriftSemantics(t *testing.T) {
	cr := harborv1alpha1.Registry{
		ObjectMeta: metav1.ObjectMeta{Name: "upstream", Namespace: testNamespace},
		Spec:       harborv1alpha1.RegistrySpec{URL: "https://registry.example.test", Type: "docker-hub", Description: "source", Insecure: true},
	}
	credential := &harborclient.RegistryCredential{Type: "basic", AccessKey: "robot-user", AccessSecret: "robot-token"}
	reconciler := &RegistryReconciler{}
	create := reconciler.buildCreateReq(cr, credential, "ca-pem")
	if create.Name != cr.Name || create.URL != cr.Spec.URL || create.Type != cr.Spec.Type || !create.Insecure || create.CACertificate != "ca-pem" || create.Credential == nil || create.Credential.AccessSecret != "robot-token" {
		t.Fatalf("registry create request = %#v", create)
	}
	update := reconciler.buildUpdateReq(cr, credential, "ca-pem")
	if update.Name != cr.Name || update.URL != cr.Spec.URL || update.CredentialType != "basic" || update.AccessKey != "robot-user" || update.AccessSecret != "robot-token" {
		t.Fatalf("registry update request = %#v", update)
	}
	withoutCredential := reconciler.buildUpdateReq(cr, nil, "")
	if withoutCredential.CredentialType != "" || withoutCredential.AccessKey != "" || withoutCredential.AccessSecret != "" {
		t.Fatalf("credential-free update request unexpectedly mutates credentials: %#v", withoutCredential)
	}

	current := harborclient.Registry{Name: cr.Name, URL: cr.Spec.URL, Type: "DOCKER-HUB", Description: cr.Spec.Description, Insecure: cr.Spec.Insecure}
	cr.Status.CredentialHash = "secret-hash"
	if registryNeedsUpdate(cr, current, "secret-hash", "") {
		t.Fatal("case-only registry type difference caused drift")
	}
	if registryNeedsUpdate(cr, current, "", "") {
		t.Fatal("omitted credentials caused a credential update")
	}
	if !registryNeedsUpdate(cr, current, "rotated-hash", "") {
		t.Fatal("credential Secret rotation was ignored")
	}
	if !registryNeedsUpdate(cr, current, "secret-hash", "new-ca") {
		t.Fatal("CA certificate drift was ignored")
	}
	current.URL = "https://other.example.test"
	if !registryNeedsUpdate(cr, current, "secret-hash", "") {
		t.Fatal("registry endpoint drift was ignored")
	}
}
