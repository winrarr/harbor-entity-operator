package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

//nolint:gocyclo // The mock verifies all scanner transitions and credential rotation in one lifecycle.
func TestScannerRegistrationReconcileLifecycle(t *testing.T) {
	ctx := context.Background()
	var remote harborclient.ScannerRegistration
	var createCalls, updateCalls, defaultCalls, deleteCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const collectionPath = "/api/v2.0/scanners"
		const itemPath = collectionPath + "/scanner-uuid"
		switch {
		case r.Method == http.MethodPost && r.URL.Path == collectionPath:
			var desired harborclient.ScannerRegistrationReq
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode scanner create: %v", err)
			}
			createCalls++
			remote = scannerFromRequest("scanner-uuid", desired, false)
			w.Header().Set("Location", itemPath)
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == itemPath:
			writeJSON(t, w, remote)
		case r.Method == http.MethodPut && r.URL.Path == itemPath:
			var desired harborclient.ScannerRegistrationReq
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode scanner update: %v", err)
			}
			updateCalls++
			remote = scannerFromRequest(remote.UUID, desired, remote.IsDefault)
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPatch && r.URL.Path == itemPath:
			var body struct {
				IsDefault bool `json:"is_default"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode scanner default update: %v", err)
			}
			defaultCalls++
			remote.IsDefault = body.IsDefault
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && r.URL.Path == itemPath:
			deleteCalls++
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected scanner request %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	connection, secret := clusterHarborTestObjects(server.URL)
	scanner := &harborv1alpha1.ScannerRegistration{
		ObjectMeta: metav1.ObjectMeta{Name: "scanner", Namespace: testNamespace},
		Spec: harborv1alpha1.ScannerRegistrationSpec{
			HarborSpecBase:   harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef()},
			URL:              "https://scanner.example.test",
			AccessCredential: "initial-token",
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, scanner)
	reconciler := &ScannerRegistrationReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: scanner.Namespace, Name: scanner.Name}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("create scanner: %v", err)
	}
	var stored harborv1alpha1.ScannerRegistration
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get created scanner: %v", err)
	}
	if stored.Status.HarborScannerID != "scanner-uuid" || stored.Status.CredentialHash != hashParts("initial-token") {
		t.Fatalf("created scanner status = %#v", stored.Status)
	}
	assertReadyCondition(t, stored.Status.Conditions)
	if !controllerutil.ContainsFinalizer(&stored, finalizerName) {
		t.Fatalf("scanner finalizers = %v, want Harbor cleanup finalizer", stored.Finalizers)
	}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op scanner reconcile: %v", err)
	}
	if createCalls != 1 || updateCalls != 0 {
		t.Fatalf("no-op scanner calls = create %d, update %d", createCalls, updateCalls)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload scanner before spec update: %v", err)
	}
	stored.Spec.Description = "refreshed scanner"
	stored.Spec.AccessCredential = "rotated-token"
	stored.Spec.Default = true
	if err := kubeClient.Update(ctx, &stored); err != nil {
		t.Fatalf("update scanner spec: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("update scanner: %v", err)
	}
	if updateCalls != 1 || defaultCalls != 1 || remote.Description != "refreshed scanner" || remote.AccessCredential != "rotated-token" || !remote.IsDefault {
		t.Fatalf("scanner drift reconciliation calls=%d/%d remote=%#v", updateCalls, defaultCalls, remote)
	}
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get scanner after update: %v", err)
	}
	if stored.Status.CredentialHash != hashParts("rotated-token") {
		t.Fatalf("scanner credential hash = %q, want rotated token hash", stored.Status.CredentialHash)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("stable scanner reconcile: %v", err)
	}
	if updateCalls != 1 || defaultCalls != 1 {
		t.Fatalf("stable scanner caused writes: update=%d default=%d", updateCalls, defaultCalls)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get scanner before deletion: %v", err)
	}
	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete scanner: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("finalize scanner deletion: %v", err)
	}
	if deleteCalls != 1 {
		t.Fatalf("scanner Harbor deletes = %d, want 1", deleteCalls)
	}
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err == nil {
		t.Fatal("scanner remains after successful finalization")
	}
}

//nolint:gocyclo // The mock verifies the label lifecycle and scope validation through Reconcile.
func TestLabelReconcileLifecycleAndProjectScopeGuards(t *testing.T) {
	ctx := context.Background()
	var remote *harborclient.Label
	var createCalls, updateCalls, deleteCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const collectionPath = "/api/v2.0/labels"
		const itemPath = collectionPath + "/17"
		switch {
		case r.Method == http.MethodPost && r.URL.Path == collectionPath:
			var desired harborclient.Label
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode label create: %v", err)
			}
			createCalls++
			desired.ID = 17
			remote = &desired
			w.Header().Set("Location", itemPath)
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == itemPath:
			if remote == nil {
				http.NotFound(w, r)
				return
			}
			writeJSON(t, w, remote)
		case r.Method == http.MethodPut && r.URL.Path == itemPath:
			var desired harborclient.Label
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode label update: %v", err)
			}
			updateCalls++
			desired.ID = 17
			remote = &desired
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && r.URL.Path == itemPath:
			deleteCalls++
			remote = nil
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected label request %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	connection, secret := clusterHarborTestObjects(server.URL)
	label := &harborv1alpha1.Label{
		ObjectMeta: metav1.ObjectMeta{Name: "release", Namespace: testNamespace},
		Spec: harborv1alpha1.LabelSpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef()},
			Description:    "release label",
			Color:          "#3366ff",
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, label)
	reconciler := &LabelReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: label.Namespace, Name: label.Name}}

	if _, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: label.Namespace, Name: "absent"}}); err != nil {
		t.Fatalf("reconcile deleted label request: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("create label: %v", err)
	}
	if createCalls != 1 || remote == nil || remote.Scope != "g" || remote.ID != 17 {
		t.Fatalf("label create calls/remote = %d/%#v, want global scope and ID 17", createCalls, remote)
	}
	var stored harborv1alpha1.Label
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get created label: %v", err)
	}
	if stored.Status.HarborLabelID != 17 {
		t.Fatalf("label Harbor ID = %d, want 17", stored.Status.HarborLabelID)
	}
	assertReadyCondition(t, stored.Status.Conditions)

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op label reconcile: %v", err)
	}
	if updateCalls != 0 {
		t.Fatalf("no-op label reconcile updated Harbor %d times", updateCalls)
	}
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload label before spec update: %v", err)
	}
	stored.Spec.Description = "updated release label"
	if err := kubeClient.Update(ctx, &stored); err != nil {
		t.Fatalf("update label spec: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("update label: %v", err)
	}
	if updateCalls != 1 || remote.Description != "updated release label" {
		t.Fatalf("label drift update calls/remote = %d/%#v", updateCalls, remote)
	}

	invalid := &harborv1alpha1.Label{
		ObjectMeta: metav1.ObjectMeta{Name: "invalid-project-label", Namespace: testNamespace},
		Spec: harborv1alpha1.LabelSpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef()},
			Scope:          "g",
			ProjectRef:     &harborv1alpha1.ProjectReference{Name: "team"},
		},
	}
	if err := kubeClient.Create(ctx, invalid); err != nil {
		t.Fatalf("create invalid project-scoped label: %v", err)
	}
	invalidReq := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: invalid.Namespace, Name: invalid.Name}}
	if _, err := reconciler.Reconcile(ctx, invalidReq); err == nil || !strings.Contains(err.Error(), "spec.scope must be 'p'") {
		t.Fatalf("scope/project reference conflict error = %v", err)
	}
	if err := kubeClient.Get(ctx, invalidReq.NamespacedName, &stored); err != nil {
		t.Fatalf("get invalid label status: %v", err)
	}
	if condition := meta.FindStatusCondition(stored.Status.Conditions, ConditionReady); condition == nil || condition.Status != metav1.ConditionFalse {
		t.Fatalf("invalid label Ready condition = %#v", condition)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get label before deletion: %v", err)
	}
	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete label: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("finalize label deletion: %v", err)
	}
	if deleteCalls != 1 || remote != nil {
		t.Fatalf("label delete calls/remote = %d/%#v, want one remote delete", deleteCalls, remote)
	}
}

func scannerFromRequest(id string, request harborclient.ScannerRegistrationReq, isDefault bool) harborclient.ScannerRegistration {
	return harborclient.ScannerRegistration{
		UUID: id, Name: request.Name, Description: request.Description, URL: request.URL,
		Auth: request.Auth, AccessCredential: request.AccessCredential,
		SkipCertVerify: request.SkipCertVerify, UseInternalAddr: request.UseInternalAddr,
		Disabled: request.Disabled, IsDefault: isDefault,
	}
}
