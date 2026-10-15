package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

func TestConfigurationReconcileAppliesSubsetAndSecretRotation(t *testing.T) {
	ctx := context.Background()
	current := map[string]json.RawMessage{
		"robot_name_prefix": json.RawMessage(`"old"`),
		"scan_all_policy":   json.RawMessage(`false`),
	}
	var updates []map[string]json.RawMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2.0/configurations" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			response := map[string]harborclient.ConfigurationItem{}
			for key, value := range current {
				response[key] = harborclient.ConfigurationItem{Value: value, Editable: true}
			}
			writeJSON(t, w, response)
		case http.MethodPut:
			var desired map[string]json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode configuration update: %v", err)
			}
			updates = append(updates, desired)
			for key, value := range desired {
				current[key] = value
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected configuration request %s %s", r.Method, r.URL)
			http.Error(w, "unexpected request", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	connection, connectionSecret := clusterHarborTestObjects(server.URL)
	settingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "configuration-values", Namespace: testNamespace},
		Data:       map[string][]byte{"scan-all": []byte("true")},
	}
	configuration := &harborv1alpha1.Configuration{
		ObjectMeta: metav1.ObjectMeta{Name: "system-settings", Namespace: testNamespace},
		Spec: harborv1alpha1.ConfigurationSpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef()},
			Settings: map[string]harborv1alpha1.ConfigurationValue{
				"robot_name_prefix": {Value: &apiextensionsv1.JSON{Raw: []byte(`"operator"`)}},
				"scan_all_policy": {ValueFrom: &harborv1alpha1.ConfigurationValueSource{SecretKeyRef: harborv1alpha1.SecretReference{
					Name: settingSecret.Name, Namespace: testNamespace, Key: "scan-all",
				}}},
			},
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, connectionSecret, settingSecret, configuration)
	reconciler := &ConfigurationReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: configuration.Namespace, Name: configuration.Name}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("apply Harbor configuration: %v", err)
	}
	var stored harborv1alpha1.Configuration
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get configuration after apply: %v", err)
	}
	if len(updates) != 1 || string(updates[0]["robot_name_prefix"]) != `"operator"` || string(updates[0]["scan_all_policy"]) != "true" {
		t.Fatalf("configuration update payloads = %#v", updates)
	}
	assertReadyCondition(t, stored.Status.Conditions)

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op Harbor configuration reconcile: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("no-op reconcile sent %d Harbor configuration updates", len(updates)-1)
	}

	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: settingSecret.Name}, settingSecret); err != nil {
		t.Fatalf("get configuration Secret before rotation: %v", err)
	}
	settingSecret.Data["scan-all"] = []byte("false")
	if err := kubeClient.Update(ctx, settingSecret); err != nil {
		t.Fatalf("rotate configuration Secret: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile rotated configuration Secret: %v", err)
	}
	if len(updates) != 2 || string(updates[1]["scan_all_policy"]) != "false" {
		t.Fatalf("Secret rotation update payloads = %#v", updates)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get configuration before deletion: %v", err)
	}
	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete Configuration: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("finalize Configuration deletion: %v", err)
	}
	if len(updates) != 2 {
		t.Fatalf("deleting Configuration changed Harbor settings: update count=%d", len(updates))
	}
}

func TestConfigurationWithoutSettingsDoesNotReadOrWriteHarbor(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("empty configuration made an unexpected Harbor request %s %s", r.Method, r.URL)
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer server.Close()
	connection, secret := clusterHarborTestObjects(server.URL)
	configuration := &harborv1alpha1.Configuration{
		ObjectMeta: metav1.ObjectMeta{Name: "empty-settings", Namespace: testNamespace},
		Spec: harborv1alpha1.ConfigurationSpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef()},
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, configuration)
	reconciler := &ConfigurationReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: configuration.Namespace, Name: configuration.Name}}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile empty configuration: %v", err)
	}
	var stored harborv1alpha1.Configuration
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get empty configuration status: %v", err)
	}
	assertReadyCondition(t, stored.Status.Conditions)
}

func TestQuotaReconcileResolvesProjectAndSynchronizesHardLimits(t *testing.T) {
	ctx := context.Background()
	var hard = map[string]int64{"storage": 1000}
	var updates []map[string]int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/quotas":
			if got := r.URL.Query().Get("reference_id"); got != "42" || r.URL.Query().Get("reference") != "project" {
				t.Errorf("quota lookup query = %s", r.URL.RawQuery)
			}
			writeJSON(t, w, []harborclient.Quota{{ID: 99, Hard: hard}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/quotas/99":
			writeJSON(t, w, harborclient.Quota{ID: 99, Hard: hard})
		case r.Method == http.MethodPut && r.URL.Path == "/api/v2.0/quotas/99":
			var body struct {
				Hard map[string]int64 `json:"hard"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode quota update: %v", err)
			}
			updates = append(updates, body.Hard)
			hard = body.Hard
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected quota request %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	connection, secret := clusterHarborTestObjects(server.URL)
	project := &harborv1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: testNamespace}, Status: harborv1alpha1.ProjectStatus{HarborProjectID: 42}}
	quota := &harborv1alpha1.Quota{
		ObjectMeta: metav1.ObjectMeta{Name: "project-quota", Namespace: testNamespace},
		Spec: harborv1alpha1.QuotaSpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef()},
			ProjectRef:     &harborv1alpha1.ProjectReference{Name: project.Name},
			Hard:           map[string]int64{"storage": 2000},
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, project, quota)
	reconciler := &QuotaReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: quota.Namespace, Name: quota.Name}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile quota: %v", err)
	}
	var stored harborv1alpha1.Quota
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get quota after reconcile: %v", err)
	}
	if stored.Status.HarborQuotaID != 99 || len(updates) != 1 || updates[0]["storage"] != 2000 {
		t.Fatalf("quota status/update payload = %d/%#v", stored.Status.HarborQuotaID, updates)
	}
	assertReadyCondition(t, stored.Status.Conditions)

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op quota reconcile: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("no-op quota reconcile sent %d updates", len(updates)-1)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload quota: %v", err)
	}
	stored.Spec.Hard = map[string]int64{"storage": 3000, "count": 75}
	if err := kubeClient.Update(ctx, &stored); err != nil {
		t.Fatalf("update quota spec: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("apply quota update: %v", err)
	}
	if len(updates) != 2 || updates[1]["storage"] != 3000 || updates[1]["count"] != 75 {
		t.Fatalf("updated quota payloads = %#v", updates)
	}
}
