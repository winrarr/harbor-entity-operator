package controller

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

func TestConfigurationValuesResolveLiteralAndSecretJSON(t *testing.T) {
	ctx := context.Background()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "settings", Namespace: testNamespace},
		Data: map[string][]byte{
			"boolean": []byte("false"),
			"object":  []byte(`{"workers":3}`),
			"text":    []byte("plain value"),
		},
	}
	kubeClient := newAdvancedUnitClient(t, secret)
	reconciler := &ConfigurationReconciler{Client: kubeClient}
	configuration := &harborv1alpha1.Configuration{ObjectMeta: metav1.ObjectMeta{Name: "settings", Namespace: testNamespace}}

	literal, err := reconciler.resolveConfigurationValue(ctx, testNamespace, "literal", harborv1alpha1.ConfigurationValue{Value: &apiextensionsv1.JSON{Raw: []byte(`{"mode":"strict"}`)}})
	if err != nil || literal.(map[string]any)["mode"] != "strict" {
		t.Fatalf("literal configuration value = %#v, %v", literal, err)
	}
	boolean, err := reconciler.resolveConfigurationValue(ctx, testNamespace, "boolean", harborv1alpha1.ConfigurationValue{ValueFrom: &harborv1alpha1.ConfigurationValueSource{SecretKeyRef: harborv1alpha1.SecretReference{Name: secret.Name, Namespace: testNamespace, Key: "boolean"}}})
	if err != nil || boolean != false {
		t.Fatalf("JSON Secret configuration value = %#v, %v; want boolean false", boolean, err)
	}
	object, err := reconciler.resolveConfigurationValue(ctx, testNamespace, "object", harborv1alpha1.ConfigurationValue{ValueFrom: &harborv1alpha1.ConfigurationValueSource{SecretKeyRef: harborv1alpha1.SecretReference{Name: secret.Name, Namespace: testNamespace, Key: "object"}}})
	if err != nil || object.(map[string]any)["workers"] != float64(3) {
		t.Fatalf("object Secret configuration value = %#v, %v", object, err)
	}
	text, err := reconciler.resolveConfigurationValue(ctx, testNamespace, "text", harborv1alpha1.ConfigurationValue{ValueFrom: &harborv1alpha1.ConfigurationValueSource{SecretKeyRef: harborv1alpha1.SecretReference{Name: secret.Name, Namespace: testNamespace, Key: "text"}}})
	if err != nil || text != "plain value" {
		t.Fatalf("plain Secret configuration value = %#v, %v", text, err)
	}

	if _, err := reconciler.resolveConfigurationValue(ctx, testNamespace, "absent", harborv1alpha1.ConfigurationValue{}); err == nil || !strings.Contains(err.Error(), "must define value or valueFrom") {
		t.Fatalf("missing configuration source error = %v", err)
	}
	if _, err := reconciler.resolveConfigurationValue(ctx, testNamespace, "invalid", harborv1alpha1.ConfigurationValue{Value: &apiextensionsv1.JSON{Raw: []byte(`{`)}}); err == nil || !strings.Contains(err.Error(), "invalid settings value") {
		t.Fatalf("invalid literal JSON error = %v", err)
	}
	if _, err := reconciler.resolveConfigurationValue(ctx, testNamespace, "missing-key", harborv1alpha1.ConfigurationValue{ValueFrom: &harborv1alpha1.ConfigurationValueSource{SecretKeyRef: harborv1alpha1.SecretReference{Name: secret.Name, Namespace: testNamespace, Key: "missing"}}}); err == nil || !strings.Contains(err.Error(), "failed to read secret") {
		t.Fatalf("missing Secret key error = %v", err)
	}

	configuration.Spec.Settings = map[string]harborv1alpha1.ConfigurationValue{
		"workers": {Value: &apiextensionsv1.JSON{Raw: []byte(`3`)}},
		"strict":  {Value: &apiextensionsv1.JSON{Raw: []byte(`true`)}},
	}
	desired, err := reconciler.buildDesiredSettings(ctx, configuration)
	if err != nil || desired["workers"] != float64(3) || desired["strict"] != true {
		t.Fatalf("combined desired settings = %#v, %v", desired, err)
	}
}

func TestConfigurationEditabilityAndJSONDriftContracts(t *testing.T) {
	desired := map[string]any{
		"editable":  map[string]any{"a": float64(1), "b": true},
		"missing":   "new value",
		"unchanged": int64(7),
	}
	current := map[string]harborclient.ConfigurationItem{
		"editable":  {Editable: true, Value: json.RawMessage(`{"b":true,"a":1}`)},
		"unchanged": {Editable: false, Value: json.RawMessage(`7`)},
		"readonly":  {Editable: false, Value: json.RawMessage(`"server"`)},
	}
	if err := ensureEditableSettings(desired, current); err == nil || !strings.Contains(err.Error(), `configuration "unchanged" is not editable`) {
		t.Fatalf("read-only desired setting error = %v", err)
	}
	delete(desired, "unchanged")
	if err := ensureEditableSettings(desired, current); err != nil {
		t.Fatalf("read-only setting not included in desired state blocked reconcile: %v", err)
	}
	if !jsonValuesEqual(map[string]any{"a": float64(1), "b": true}, json.RawMessage(`{"b":true,"a":1}`)) {
		t.Fatal("equivalent JSON objects with different key order caused drift")
	}
	if jsonValuesEqual(map[string]any{"workers": float64(4)}, json.RawMessage(`{"workers":5}`)) {
		t.Fatal("different configuration values were considered equal")
	}
	if jsonValuesEqual("value", json.RawMessage(`not-json`)) {
		t.Fatal("invalid current JSON was considered equal")
	}
	if configurationNeedsUpdate(map[string]any{}, current) {
		t.Fatal("empty desired subset should not reset unrelated Harbor configuration")
	}
	if !configurationNeedsUpdate(map[string]any{"new": "value"}, current) {
		t.Fatal("missing desired Harbor configuration was ignored")
	}
	if configurationNeedsUpdate(map[string]any{"editable": map[string]any{"b": true, "a": float64(1)}}, current) {
		t.Fatal("semantically equal desired configuration caused an update")
	}
	if !configurationNeedsUpdate(map[string]any{"editable": map[string]any{"a": float64(9)}}, current) {
		t.Fatal("configuration drift was ignored")
	}
	if jsonValuesEqual(make(chan int), json.RawMessage(`null`)) {
		t.Fatal("unmarshalable desired value was considered equal")
	}
	if jsonValuesEqual("value", nil) {
		t.Fatal("empty current JSON was considered equal")
	}
	if !configurationNeedsUpdate(map[string]any{"editable": map[string]any{"a": make(chan int)}}, current) {
		t.Fatal("unmarshalable desired value was not treated as drift")
	}
}
