package controller

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
)

func TestConnectionResolutionHonorsReferenceKindsAndForcedConnection(t *testing.T) {
	ctx := context.Background()
	cluster, secret := clusterHarborTestObjects("https://cluster.example.test")
	namespaced := &harborv1alpha1.HarborConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "tenant-harbor", Namespace: testNamespace},
		Spec:       cluster.Spec,
	}
	client := newAdvancedUnitClient(t, cluster, namespaced, secret)

	clusterConfig, err := resolveHarborConnection(ctx, OperatorOptions{}, client, testNamespace, clusterHarborRef())
	if err != nil || clusterConfig.kind != harborv1alpha1.HarborConnectionReferenceKindCluster || clusterConfig.namespace != "" || clusterConfig.baseURL != cluster.Spec.BaseURL {
		t.Fatalf("cluster connection resolution = %#v, %v", clusterConfig, err)
	}
	namespacedConfig, err := resolveHarborConnection(ctx, OperatorOptions{}, client, testNamespace, &harborv1alpha1.HarborConnectionReference{Name: namespaced.Name})
	if err != nil || namespacedConfig.kind != harborv1alpha1.HarborConnectionReferenceKindNamespaced || namespacedConfig.namespace != testNamespace {
		t.Fatalf("default namespaced connection resolution = %#v, %v", namespacedConfig, err)
	}
	forced := OperatorOptions{forcedHarborConnection: cluster.Name}
	forcedConfig, err := resolveHarborConnection(ctx, forced, client, "ignored", nil)
	if err != nil || forcedConfig.name != cluster.Name || forcedConfig.kind != harborv1alpha1.HarborConnectionReferenceKindCluster {
		t.Fatalf("forced connection resolution = %#v, %v", forcedConfig, err)
	}
	if _, err := resolveHarborConnection(ctx, forced, client, testNamespace, &harborv1alpha1.HarborConnectionReference{Name: namespaced.Name}); err == nil || !strings.Contains(err.Error(), "must be omitted or reference ClusterHarborConnection") {
		t.Fatalf("mismatched forced connection error = %v", err)
	}
	if _, err := resolveHarborConnection(ctx, OperatorOptions{}, client, testNamespace, nil); err == nil || !strings.Contains(err.Error(), "is required") {
		t.Fatalf("missing connection reference error = %v", err)
	}
	if _, err := resolveHarborConnection(ctx, OperatorOptions{}, client, testNamespace, &harborv1alpha1.HarborConnectionReference{Name: "bad", Kind: "Other"}); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("unsupported connection kind error = %v", err)
	}
}

func TestConnectionBindingIsPersistedBeforeHarborClientAndCannotChange(t *testing.T) {
	ctx := context.Background()
	connection, secret := clusterHarborTestObjects("https://cluster.example.test")
	allowlist := &harborv1alpha1.HarborCVEAllowlist{
		ObjectMeta: metav1.ObjectMeta{Name: "system-allowlist"},
		Spec:       harborv1alpha1.HarborCVEAllowlistSpec{HarborConnectionRef: clusterHarborRef()},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, allowlist)
	var stored harborv1alpha1.HarborCVEAllowlist
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: allowlist.Name}, &stored); err != nil {
		t.Fatalf("get allowlist: %v", err)
	}
	harborClient, err := getHarborClientForObject(ctx, OperatorOptions{}, kubeClient, &stored, &stored.Status.HarborStatusBase, "", stored.Spec.HarborConnectionRef)
	if err != nil || harborClient == nil {
		t.Fatalf("first Harbor client construction = %v, %v", harborClient, err)
	}
	if stored.Status.ResolvedHarborConnection == nil || stored.Status.ResolvedHarborConnection.UID != string(connection.UID) {
		t.Fatalf("resolved connection binding was not recorded: %#v", stored.Status.ResolvedHarborConnection)
	}
	var persisted harborv1alpha1.HarborCVEAllowlist
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: allowlist.Name}, &persisted); err != nil {
		t.Fatalf("get allowlist after binding: %v", err)
	}
	if persisted.Status.ResolvedHarborConnection == nil || persisted.Status.ResolvedHarborConnection.Name != connection.Name {
		t.Fatalf("persisted connection binding = %#v", persisted.Status.ResolvedHarborConnection)
	}

	persisted.Status.ResolvedHarborConnection.UID = "previous-uid"
	if _, err := getHarborClientForObject(ctx, OperatorOptions{}, kubeClient, &persisted, &persisted.Status.HarborStatusBase, "", persisted.Spec.HarborConnectionRef); err == nil || !strings.Contains(err.Error(), "refusing to modify Harbor") {
		t.Fatalf("connection identity change error = %v", err)
	}
}

func TestConnectionCABundleAndHarborClientTLSValidation(t *testing.T) {
	ctx := context.Background()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "connection-ca", Namespace: testNamespace},
		Data:       map[string][]byte{"ca.crt": []byte("PEM contents")},
	}
	kubeClient := newAdvancedUnitClient(t, secret)
	inline, err := resolveConnectionCABundle(ctx, OperatorOptions{}, kubeClient, &connectionConfig{caBundle: "inline PEM"})
	if err != nil || inline != "inline PEM" {
		t.Fatalf("inline CA bundle = %q, %v", inline, err)
	}
	ref := harborv1alpha1.SecretReference{Name: secret.Name, Namespace: testNamespace}
	fromSecret, err := resolveConnectionCABundle(ctx, OperatorOptions{}, kubeClient, &connectionConfig{namespace: testNamespace, caBundleSecretRef: &ref})
	if err != nil || fromSecret != "PEM contents" {
		t.Fatalf("Secret CA bundle = %q, %v", fromSecret, err)
	}
	if _, err := resolveConnectionCABundle(ctx, OperatorOptions{}, kubeClient, &connectionConfig{namespace: testNamespace, caBundle: "inline", caBundleSecretRef: &ref}); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("inline and Secret CA conflict error = %v", err)
	}
	missing := harborv1alpha1.SecretReference{Name: "missing", Namespace: testNamespace}
	if _, err := resolveConnectionCABundle(ctx, OperatorOptions{}, kubeClient, &connectionConfig{namespace: testNamespace, caBundleSecretRef: &missing}); err == nil || !strings.Contains(err.Error(), "failed to read caBundleSecretRef") {
		t.Fatalf("missing CA Secret error = %v", err)
	}
	if _, err := newHarborClient(OperatorOptions{}, "https://harbor.example.test", "user", "pass", "not a certificate"); err == nil || !strings.Contains(err.Error(), "invalid caBundle") {
		t.Fatalf("invalid CA bundle error = %v", err)
	}
	hc, err := newHarborClient(OperatorOptions{harborRequestTimeout: 2 * time.Second}, "https://harbor.example.test/", "user", "pass", "")
	if err != nil || hc.BaseURL != "https://harbor.example.test" || hc.HTTPClient.Timeout != 2*time.Second {
		t.Fatalf("Harbor client configuration = %#v, %v", hc, err)
	}
}

func TestBaseURLAndConnectionChangeErrorsAreActionable(t *testing.T) {
	for _, value := range []string{"https://harbor.example.test", "http://localhost:8080"} {
		if err := validateBaseURL(value); err != nil {
			t.Errorf("validateBaseURL(%q): %v", value, err)
		}
	}
	for _, value := range []string{"harbor.example.test", "://bad"} {
		if err := validateBaseURL(value); err == nil {
			t.Errorf("validateBaseURL(%q) accepted missing or malformed URL", value)
		}
	}
	err := &harborConnectionChangedError{
		previous: harborv1alpha1.HarborConnectionBinding{Kind: "ClusterHarborConnection", Name: "old", UID: "old-uid"},
		current:  harborv1alpha1.HarborConnectionBinding{Kind: "ClusterHarborConnection", Name: "new", UID: "new-uid"},
	}
	message := err.Error()
	for _, expected := range []string{"old-uid", "new-uid", "refusing to modify Harbor"} {
		if !strings.Contains(message, expected) {
			t.Errorf("connection change message %q omits %q", message, expected)
		}
	}
}

func TestRemoteNotFoundRequeuesWithStatusAndIdentityReset(t *testing.T) {
	ctx := context.Background()
	project := &harborv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: testNamespace},
		Status:     harborv1alpha1.ProjectStatus{HarborProjectID: 32},
	}
	kubeClient := newAdvancedUnitClient(t, project)
	var stored harborv1alpha1.Project
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: project.Name}, &stored); err != nil {
		t.Fatalf("get project: %v", err)
	}
	reset := false
	result, err := requeueOnRemoteNotFound(ctx, kubeClient, &stored, &stored.Status.HarborStatusBase, stored.Generation, func() {
		reset = true
		stored.Status.HarborProjectID = 0
	}, "Project missing from Harbor")
	if err != nil || !reset || result.RequeueAfter != dependencyRequeueInterval {
		t.Fatalf("remote-not-found result/reset/error = %#v/%t/%v", result, reset, err)
	}
	if immediate := requeueImmediately(); immediate.RequeueAfter <= 0 || immediate.RequeueAfter >= time.Second {
		t.Fatalf("immediate requeue interval = %s, want a short positive delay", immediate.RequeueAfter)
	}
	var persisted harborv1alpha1.Project
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: project.Name}, &persisted); err != nil {
		t.Fatalf("get project after reset: %v", err)
	}
	if persisted.Status.HarborProjectID != 0 {
		t.Fatalf("Harbor project ID was not reset: %d", persisted.Status.HarborProjectID)
	}
	condition := meta.FindStatusCondition(persisted.Status.Conditions, ConditionReady)
	if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "NotFound" || condition.Message != "Project missing from Harbor" {
		t.Fatalf("Ready condition after remote deletion = %#v", condition)
	}
}

func TestLoadResourceDistinguishesMissingAndOperationalErrors(t *testing.T) {
	ctx := context.Background()
	project := &harborv1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "existing", Namespace: testNamespace}}
	kubeClient := newAdvancedUnitClient(t, project)
	var loaded harborv1alpha1.Project
	found, err := loadResource(ctx, kubeClient, types.NamespacedName{Namespace: testNamespace, Name: "existing"}, &loaded, logr.Discard())
	if err != nil || !found || loaded.Name != project.Name {
		t.Fatalf("existing load = %t, %#v, %v", found, loaded, err)
	}
	found, err = loadResource(ctx, kubeClient, types.NamespacedName{Namespace: testNamespace, Name: "missing"}, &loaded, logr.Discard())
	if err != nil || found {
		t.Fatalf("missing load = %t, %v; want absent without error", found, err)
	}
	found, err = loadResource(ctx, errorGetClient{Client: kubeClient, err: errors.New("storage unavailable")}, types.NamespacedName{Namespace: testNamespace, Name: "existing"}, &loaded, logr.Discard())
	if found || err == nil || err.Error() != "storage unavailable" {
		t.Fatalf("operational load failure = %t, %v", found, err)
	}
}

type errorGetClient struct {
	client.Client
	err error
}

func (c errorGetClient) Get(context.Context, types.NamespacedName, client.Object, ...client.GetOption) error {
	return c.err
}
