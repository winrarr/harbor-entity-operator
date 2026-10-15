package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func newUnitHarborClient(t *testing.T, handler http.Handler) *harborclient.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return harborclient.NewWithHTTPClient(server.URL, "", "", server.Client())
}

func TestImmutableRuleDiscoveryAdoptionAndMissingID(t *testing.T) {
	ctx := context.Background()
	desired := harborclient.ImmutableRule{Priority: 1, Action: "immutable", Template: "always", TagSelectors: []harborclient.ImmutableSelector{{Kind: "doublestar", Pattern: "release-*"}}}
	var listError bool
	hc := newUnitHarborClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2.0/projects/project/immutabletagrules" {
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
			return
		}
		if listError {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		writeJSON(t, w, []harborclient.ImmutableRule{
			{ID: 11, Priority: 1, Action: "immutable", Template: "always", TagSelectors: []harborclient.ImmutableSelector{{Kind: "doublestar", Pattern: "other-*"}}},
			{ID: 42, Priority: 1, Action: "immutable", Template: "always", TagSelectors: []harborclient.ImmutableSelector{{Kind: "doublestar", Pattern: "release-*"}}},
		})
	}))
	cr := &harborv1alpha1.ImmutableTagRule{ObjectMeta: metav1.ObjectMeta{Name: "release", Namespace: testNamespace}}
	kubeClient := newAdvancedUnitClient(t, cr)
	reconciler := &ImmutableTagRuleReconciler{Client: kubeClient}

	if id, err := reconciler.findMatchingRuleID(ctx, hc, "project", desired); err != nil || id != 42 {
		t.Fatalf("matching immutable rule ID = %d, %v; want 42", id, err)
	}
	if got, err := reconciler.getImmutableRule(ctx, hc, "project", 42); err != nil || got.ID != 42 {
		t.Fatalf("immutable rule lookup = %#v, %v", got, err)
	}
	if _, err := reconciler.getImmutableRule(ctx, hc, "project", 999); err == nil || !harborclient.IsNotFound(err) {
		t.Fatalf("missing immutable rule error = %v, want Harbor not-found", err)
	}
	adopted, err := reconciler.adoptExisting(ctx, hc, "project", cr, desired)
	if err != nil || !adopted || cr.Status.HarborImmutableRuleID != 42 {
		t.Fatalf("immutable rule adoption = %t, ID=%d, err=%v", adopted, cr.Status.HarborImmutableRuleID, err)
	}
	var stored harborv1alpha1.ImmutableTagRule
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: cr.Namespace, Name: cr.Name}, &stored); err != nil || stored.Status.HarborImmutableRuleID != 42 {
		t.Fatalf("persisted immutable rule ID = %d, %v", stored.Status.HarborImmutableRuleID, err)
	}
	if selectors := toImmutableScopeSelectors(map[string][]harborv1alpha1.ImmutableSelector{"repository": {{Kind: "doublestar", Pattern: "team/*"}}}); len(selectors["repository"]) != 1 || selectors["repository"][0].Pattern != "team/*" {
		t.Fatalf("scope selector conversion = %#v", selectors)
	}

	listError = true
	if _, err := reconciler.findMatchingRuleID(ctx, hc, "project", desired); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("immutable rule list error = %v, want Harbor API failure", err)
	}
}

func TestLabelAdoptionMatchesHarborIdentityAndProjectScope(t *testing.T) {
	ctx := context.Background()
	var listedProjectID string
	hc := newUnitHarborClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2.0/labels" || r.Method != http.MethodGet {
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
			return
		}
		listedProjectID = r.URL.Query().Get("project_id")
		writeJSON(t, w, []harborclient.Label{
			{ID: 10, Name: "Release", Scope: "p", ProjectID: 7},
			{ID: 11, Name: "release", Scope: "p", ProjectID: 42},
		})
	}))
	cr := &harborv1alpha1.Label{ObjectMeta: metav1.ObjectMeta{Name: "release", Namespace: testNamespace}}
	kubeClient := newAdvancedUnitClient(t, cr)
	reconciler := &LabelReconciler{Client: kubeClient}
	projectID := 42
	desired := harborclient.Label{Name: "release", Scope: "p", ProjectID: projectID}
	adopted, err := reconciler.adoptExisting(ctx, hc, cr, desired)
	if err != nil || !adopted || cr.Status.HarborLabelID != 11 || listedProjectID != "42" {
		t.Fatalf("project label adoption = %t, ID=%d, query project=%q, err=%v", adopted, cr.Status.HarborLabelID, listedProjectID, err)
	}
	var stored harborv1alpha1.Label
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: cr.Namespace, Name: cr.Name}, &stored); err != nil || stored.Status.HarborLabelID != 11 {
		t.Fatalf("persisted label ID = %d, %v", stored.Status.HarborLabelID, err)
	}

	cr2 := &harborv1alpha1.Label{ObjectMeta: metav1.ObjectMeta{Name: "release", Namespace: "other"}}
	kubeClient = newAdvancedUnitClient(t, cr2)
	reconciler.Client = kubeClient
	adopted, err = reconciler.adoptExisting(ctx, hc, cr2, harborclient.Label{Name: "release", Scope: "g"})
	if err != nil || adopted || cr2.Status.HarborLabelID != 0 {
		t.Fatalf("global label adoption accepted project-only matches: adopted=%t ID=%d err=%v", adopted, cr2.Status.HarborLabelID, err)
	}
}

//nolint:gocyclo // The mock verifies all observable transitions through one reconciliation lifecycle.
func TestLabelReconcileCreateNoopUpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	var remote *harborclient.Label
	var createCount, updateCount, deleteCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2.0/labels":
			var desired harborclient.Label
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode label create: %v", err)
			}
			createCount++
			desired.ID = 9
			remote = &desired
			w.Header().Set("Location", "/api/v2.0/labels/9")
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/labels/9":
			if remote == nil {
				http.NotFound(w, r)
				return
			}
			writeJSON(t, w, remote)
		case r.Method == http.MethodPut && r.URL.Path == "/api/v2.0/labels/9":
			var desired harborclient.Label
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode label update: %v", err)
			}
			updateCount++
			desired.ID = 9
			remote = &desired
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v2.0/labels/9":
			deleteCount++
			remote = nil
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	connection, secret := clusterHarborTestObjects(server.URL)
	label := &harborv1alpha1.Label{
		ObjectMeta: metav1.ObjectMeta{Name: "release", Namespace: testNamespace},
		Spec: harborv1alpha1.LabelSpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef(), DeletionPolicy: harborv1alpha1.DeletionPolicyDelete},
			Color:          "#198754",
			Description:    "stable releases",
			Scope:          "g",
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, label)
	reconciler := &LabelReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: label.Namespace, Name: label.Name}}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("create label: %v", err)
	}
	var stored harborv1alpha1.Label
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get created label: %v", err)
	}
	if createCount != 1 || stored.Status.HarborLabelID != 9 || remote == nil || remote.Color != "#198754" {
		t.Fatalf("label create calls/status/remote = %d/%d/%#v", createCount, stored.Status.HarborLabelID, remote)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op label reconcile: %v", err)
	}
	if updateCount != 0 {
		t.Fatalf("no-op label reconcile updated Harbor %d times", updateCount)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload label before spec update: %v", err)
	}
	stored.Spec.Color = "#dc3545"
	if err := kubeClient.Update(ctx, &stored); err != nil {
		t.Fatalf("update label spec: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("update label: %v", err)
	}
	if updateCount != 1 || remote == nil || remote.Color != "#dc3545" {
		t.Fatalf("label update calls/remote = %d/%#v", updateCount, remote)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload label before deletion: %v", err)
	}
	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete label: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("finalize label deletion: %v", err)
	}
	if deleteCount != 1 || remote != nil {
		t.Fatalf("label delete calls/remote = %d/%#v", deleteCount, remote)
	}
}

func TestRegistryDeleteAndAdoptionAreIdempotent(t *testing.T) {
	ctx := context.Background()
	var deleted int
	hc := newUnitHarborClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/registries":
			writeJSON(t, w, []harborclient.Registry{{ID: 17, Name: "dockerhub"}})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v2.0/registries/17":
			deleted++
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	cr := &harborv1alpha1.Registry{ObjectMeta: metav1.ObjectMeta{Name: "dockerhub", Namespace: testNamespace}}
	kubeClient := newAdvancedUnitClient(t, cr)
	reconciler := &RegistryReconciler{Client: kubeClient}
	adopted, err := reconciler.adoptExisting(ctx, hc, cr)
	if err != nil || !adopted || cr.Status.HarborRegistryID != 17 {
		t.Fatalf("registry adoption = %t, ID=%d, err=%v", adopted, cr.Status.HarborRegistryID, err)
	}
	var stored harborv1alpha1.Registry
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: cr.Namespace, Name: cr.Name}, &stored); err != nil || stored.Status.HarborRegistryID != 17 {
		t.Fatalf("persisted registry ID = %d, %v", stored.Status.HarborRegistryID, err)
	}
	if err := reconciler.deleteRegistry(ctx, hc, &stored); err != nil || deleted != 1 {
		t.Fatalf("delete already-absent registry = %v, delete requests=%d", err, deleted)
	}
	stored.Status.HarborRegistryID = 0
	if err := reconciler.deleteRegistry(ctx, hc, &stored); err != nil || deleted != 1 {
		t.Fatalf("delete registry without remote ID = %v, delete requests=%d", err, deleted)
	}
}

func TestRegistryReconcileCreateNoopUpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	var remote *harborclient.Registry
	var createCount, updateCount, deleteCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2.0/registries":
			var desired harborclient.CreateRegistryRequest
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode registry create: %v", err)
			}
			createCount++
			remote = &harborclient.Registry{ID: 17, URL: desired.URL, Name: desired.Name, Description: desired.Description, Type: desired.Type, Insecure: desired.Insecure, CACertificate: desired.CACertificate, Credential: desired.Credential}
			w.Header().Set("Location", "/api/v2.0/registries/17")
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/registries/17":
			if remote == nil {
				http.NotFound(w, r)
				return
			}
			writeJSON(t, w, remote)
		case r.Method == http.MethodPut && r.URL.Path == "/api/v2.0/registries/17":
			var desired harborclient.UpdateRegistryRequest
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode registry update: %v", err)
			}
			updateCount++
			remote.Name = desired.Name
			remote.Description = desired.Description
			remote.URL = desired.URL
			remote.Insecure = desired.Insecure
			remote.CACertificate = desired.CACertificate
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v2.0/registries/17":
			deleteCount++
			remote = nil
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	connection, passwordSecret := clusterHarborTestObjects(server.URL)
	registry := &harborv1alpha1.Registry{
		ObjectMeta: metav1.ObjectMeta{Name: "upstream", Namespace: testNamespace},
		Spec: harborv1alpha1.RegistrySpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef(), DeletionPolicy: harborv1alpha1.DeletionPolicyDelete},
			URL:            "https://registry.example.test",
			Type:           "docker-hub",
			Description:    "central upstream",
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, passwordSecret, registry)
	reconciler := &RegistryReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: registry.Namespace, Name: registry.Name}}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("create registry: %v", err)
	}
	var stored harborv1alpha1.Registry
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get registry after create: %v", err)
	}
	if createCount != 1 || stored.Status.HarborRegistryID != 17 || remote == nil || remote.URL != registry.Spec.URL {
		t.Fatalf("registry create calls/status/remote = %d/%d/%#v", createCount, stored.Status.HarborRegistryID, remote)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op registry reconcile: %v", err)
	}
	if updateCount != 0 {
		t.Fatalf("no-op registry reconcile updated Harbor %d times", updateCount)
	}
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload registry before spec update: %v", err)
	}
	stored.Spec.Description = "updated upstream description"
	if err := kubeClient.Update(ctx, &stored); err != nil {
		t.Fatalf("update registry spec: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("update registry: %v", err)
	}
	if updateCount != 1 || remote.Description != "updated upstream description" {
		t.Fatalf("registry update calls/remote = %d/%#v", updateCount, remote)
	}
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload registry before deletion: %v", err)
	}
	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete registry: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("finalize registry deletion: %v", err)
	}
	if deleteCount != 1 || remote != nil {
		t.Fatalf("registry delete calls/remote = %d/%#v", deleteCount, remote)
	}
}

func TestUserAdoptionAndDeleteIgnoreAlreadyMissingUser(t *testing.T) {
	ctx := context.Background()
	var query string
	deleteCount := 0
	hc := newUnitHarborClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/users":
			query = r.URL.Query().Get("q")
			writeJSON(t, w, []harborclient.User{{UserID: 31, Username: "Alice"}})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v2.0/users/31":
			deleteCount++
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	cr := &harborv1alpha1.User{ObjectMeta: metav1.ObjectMeta{Name: "alice", Namespace: testNamespace}}
	kubeClient := newAdvancedUnitClient(t, cr)
	reconciler := &UserReconciler{Client: kubeClient}
	adopted, err := reconciler.adoptExisting(ctx, hc, cr)
	if err != nil || !adopted || cr.Status.HarborUserID != 31 || query != "username=alice" {
		t.Fatalf("user adoption = %t, ID=%d, query=%q, err=%v", adopted, cr.Status.HarborUserID, query, err)
	}
	if err := reconciler.deleteUser(ctx, hc, cr); err != nil || deleteCount != 1 {
		t.Fatalf("delete already-absent user = %v, requests=%d", err, deleteCount)
	}
	cr.Status.HarborUserID = 0
	if err := reconciler.deleteUser(ctx, hc, cr); err != nil || deleteCount != 1 {
		t.Fatalf("delete user without Harbor ID = %v, requests=%d", err, deleteCount)
	}
	if userNeedsUpdate(harborclient.CreateUserRequest{Email: "a@example.test", Realname: "Alice", Comment: "same", Password: "new"}, harborclient.User{Email: "a@example.test", Realname: "Alice", Comment: "same"}) {
		t.Fatal("password-only change should not request Harbor user update because Harbor does not expose current passwords")
	}
}

//nolint:gocyclo // The mock verifies all observable transitions through one reconciliation lifecycle.
func TestUserReconcileCreatesUpdatesAndFinalizesDeletion(t *testing.T) {
	ctx := context.Background()
	var remote *harborclient.User
	var createCount, updateCount, deleteCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2.0/users":
			var desired harborclient.CreateUserRequest
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode user create: %v", err)
			}
			createCount++
			remote = &harborclient.User{UserID: 5, Username: desired.Username, Email: desired.Email, Realname: desired.Realname, Comment: desired.Comment}
			w.Header().Set("Location", "/api/v2.0/users/5")
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/users/5":
			if remote == nil {
				http.NotFound(w, r)
				return
			}
			writeJSON(t, w, remote)
		case r.Method == http.MethodPut && r.URL.Path == "/api/v2.0/users/5":
			var desired harborclient.CreateUserRequest
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode user update: %v", err)
			}
			updateCount++
			remote.Email = desired.Email
			remote.Realname = desired.Realname
			remote.Comment = desired.Comment
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v2.0/users/5":
			deleteCount++
			remote = nil
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	connection, passwordSecret := clusterHarborTestObjects(server.URL)
	userSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "user-password", Namespace: testNamespace}, Data: map[string][]byte{"password": []byte("initial-password")}}
	user := &harborv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "alice", Namespace: testNamespace},
		Spec: harborv1alpha1.UserSpec{
			HarborSpecBase:    harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef(), DeletionPolicy: harborv1alpha1.DeletionPolicyDelete},
			Email:             "alice@example.test",
			Comment:           "initial account",
			PasswordSecretRef: harborv1alpha1.SecretReference{Name: userSecret.Name, Key: "password"},
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, passwordSecret, userSecret, user)
	reconciler := &UserReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: user.Namespace, Name: user.Name}}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("create user: %v", err)
	}
	var stored harborv1alpha1.User
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get user after create: %v", err)
	}
	if createCount != 1 || stored.Status.HarborUserID != 5 || remote == nil || remote.Username != "alice" || remote.Realname != "alice" {
		t.Fatalf("user create calls/status/remote = %d/%d/%#v", createCount, stored.Status.HarborUserID, remote)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op user reconcile: %v", err)
	}
	if updateCount != 0 {
		t.Fatalf("no-op user reconcile updated Harbor %d times", updateCount)
	}
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload user before spec update: %v", err)
	}
	stored.Spec.Comment = "updated account"
	if err := kubeClient.Update(ctx, &stored); err != nil {
		t.Fatalf("update user spec: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("update user: %v", err)
	}
	if updateCount != 1 || remote.Comment != "updated account" {
		t.Fatalf("user update calls/remote = %d/%#v", updateCount, remote)
	}
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload user before deletion: %v", err)
	}
	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("finalize user deletion: %v", err)
	}
	if deleteCount != 1 || remote != nil {
		t.Fatalf("user deletion calls/remote = %d/%#v", deleteCount, remote)
	}
}

func TestScannerAdoptionCredentialAndDriftComparison(t *testing.T) {
	ctx := context.Background()
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "scanner-auth", Namespace: testNamespace}, Data: map[string][]byte{"accessCredential": []byte("scanner-token")}}
	cr := &harborv1alpha1.ScannerRegistration{ObjectMeta: metav1.ObjectMeta{Name: "scanner", Namespace: testNamespace}, Spec: harborv1alpha1.ScannerRegistrationSpec{AccessCredentialSecretRef: &harborv1alpha1.SecretReference{Name: secret.Name}}}
	kubeClient := newAdvancedUnitClient(t, secret, cr)
	reconciler := &ScannerRegistrationReconciler{Client: kubeClient}
	credential, firstHash, err := reconciler.resolveCredential(ctx, cr)
	if err != nil || credential != "scanner-token" || firstHash == "" {
		t.Fatalf("scanner credential/hash = %q/%q, %v", credential, firstHash, err)
	}
	cr.Spec.AccessCredential = "literal"
	if _, _, err := reconciler.resolveCredential(ctx, cr); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("literal plus Secret scanner credential error = %v", err)
	}
	cr.Spec.AccessCredential = ""
	var requests int
	hc := newUnitHarborClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2.0/scanners" {
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
			return
		}
		requests++
		writeJSON(t, w, []harborclient.ScannerRegistration{{UUID: "scan-uuid", Name: "SCANNER"}})
	}))
	adopted, err := reconciler.adoptExisting(ctx, hc, cr)
	if err != nil || !adopted || cr.Status.HarborScannerID != "scan-uuid" || requests != 1 {
		t.Fatalf("scanner adoption = %t, ID=%q, calls=%d, err=%v", adopted, cr.Status.HarborScannerID, requests, err)
	}
	desired := harborclient.ScannerRegistrationReq{Name: "scanner", URL: "https://scan.example.test", AccessCredential: "secret"}
	current := &harborclient.ScannerRegistration{Name: "scanner", URL: desired.URL, AccessCredential: "different server value"}
	if scannerNeedsUpdate(desired, current) {
		t.Fatal("credential rotation should be driven by the stored credential hash, not Harbor's possibly redacted value")
	}
	current.Disabled = true
	if !scannerNeedsUpdate(desired, current) || !scannerNeedsUpdate(desired, nil) {
		t.Fatal("scanner desired-state drift or missing scanner was ignored")
	}
}

//nolint:gocyclo // The mock verifies credential rotation and defaulting within one reconciliation lifecycle.
func TestScannerReconcileCreatesRotatesCredentialsSetsDefaultAndNoops(t *testing.T) {
	ctx := context.Background()
	var remote harborclient.ScannerRegistration
	var createCount, updateCount, defaultCount int
	hc := newUnitHarborClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2.0/scanners":
			var request harborclient.ScannerRegistrationReq
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode scanner create request: %v", err)
			}
			createCount++
			remote = harborclient.ScannerRegistration{UUID: "scan-uuid", Name: request.Name, Description: request.Description, URL: request.URL, Auth: request.Auth, AccessCredential: request.AccessCredential, SkipCertVerify: request.SkipCertVerify, UseInternalAddr: request.UseInternalAddr, Disabled: request.Disabled}
			w.Header().Set("Location", "/api/v2.0/scanners/scan-uuid")
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/scanners/scan-uuid":
			writeJSON(t, w, remote)
		case r.Method == http.MethodPut && r.URL.Path == "/api/v2.0/scanners/scan-uuid":
			var request harborclient.ScannerRegistrationReq
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode scanner update request: %v", err)
			}
			updateCount++
			remote = harborclient.ScannerRegistration{UUID: remote.UUID, Name: request.Name, Description: request.Description, URL: request.URL, Auth: request.Auth, AccessCredential: request.AccessCredential, SkipCertVerify: request.SkipCertVerify, UseInternalAddr: request.UseInternalAddr, Disabled: request.Disabled, IsDefault: remote.IsDefault}
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v2.0/scanners/scan-uuid":
			var request struct {
				IsDefault bool `json:"is_default"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode scanner default request: %v", err)
			}
			defaultCount++
			remote.IsDefault = request.IsDefault
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	scanner := &harborv1alpha1.ScannerRegistration{ObjectMeta: metav1.ObjectMeta{Name: "scanner", Namespace: testNamespace}, Spec: harborv1alpha1.ScannerRegistrationSpec{URL: "https://scanner.example.test", AccessCredential: "first-token"}}
	kubeClient := newAdvancedUnitClient(t, scanner)
	reconciler := &ScannerRegistrationReconciler{Client: kubeClient}
	if _, err := reconciler.reconcileScannerRegistration(ctx, hc, scanner); err != nil {
		t.Fatalf("create scanner: %v", err)
	}
	if createCount != 1 || updateCount != 0 || scanner.Status.HarborScannerID != "scan-uuid" || scanner.Status.CredentialHash != hashParts("first-token") {
		t.Fatalf("scanner create calls/status = create %d update %d status %#v", createCount, updateCount, scanner.Status)
	}

	var updatedScanner harborv1alpha1.ScannerRegistration
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: scanner.Namespace, Name: scanner.Name}, &updatedScanner); err != nil {
		t.Fatalf("get scanner before spec update: %v", err)
	}
	updatedScanner.Spec.Description = "scanner with refreshed token"
	updatedScanner.Spec.AccessCredential = "rotated-token"
	updatedScanner.Spec.Default = true
	if err := kubeClient.Update(ctx, &updatedScanner); err != nil {
		t.Fatalf("update scanner spec: %v", err)
	}
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: scanner.Namespace, Name: scanner.Name}, &updatedScanner); err != nil {
		t.Fatalf("reload scanner after spec update: %v", err)
	}
	scanner = &updatedScanner
	if _, err := reconciler.reconcileScannerRegistration(ctx, hc, scanner); err != nil {
		t.Fatalf("update scanner and set default: %v", err)
	}
	if createCount != 1 || updateCount != 1 || defaultCount != 1 || remote.AccessCredential != "rotated-token" || scanner.Status.CredentialHash != hashParts("rotated-token") || !remote.IsDefault {
		t.Fatalf("scanner update calls/remote/status = create %d update %d default %d remote %#v status %#v", createCount, updateCount, defaultCount, remote, scanner.Status)
	}
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: scanner.Namespace, Name: scanner.Name}, &updatedScanner); err != nil {
		t.Fatalf("reload scanner before no-op reconcile: %v", err)
	}
	scanner = &updatedScanner
	if _, err := reconciler.reconcileScannerRegistration(ctx, hc, scanner); err != nil {
		t.Fatalf("no-op scanner reconcile: %v", err)
	}
	if createCount != 1 || updateCount != 1 || defaultCount != 1 {
		t.Fatalf("no-op scanner reconcile made Harbor calls: create %d update %d default %d", createCount, updateCount, defaultCount)
	}
}

func TestWebhookPolicyAdoptionUsesNameWithinResolvedProject(t *testing.T) {
	ctx := context.Background()
	var gotPath string
	hc := newUnitHarborClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2.0/projects/team/webhook/policies" {
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
			return
		}
		writeJSON(t, w, []harborclient.WebhookPolicy{{ID: 8, Name: "Notifications"}})
	}))
	cr := &harborv1alpha1.WebhookPolicy{ObjectMeta: metav1.ObjectMeta{Name: "notifications", Namespace: testNamespace}}
	kubeClient := newAdvancedUnitClient(t, cr)
	reconciler := &WebhookPolicyReconciler{Client: kubeClient}
	adopted, err := reconciler.adoptExisting(ctx, hc, "team", cr)
	if err != nil || !adopted || cr.Status.HarborWebhookPolicyID != 8 || gotPath == "" {
		t.Fatalf("webhook policy adoption = %t, ID=%d, path=%q, err=%v", adopted, cr.Status.HarborWebhookPolicyID, gotPath, err)
	}
}

//nolint:gocyclo // The mock verifies secret-driven updates and cleanup through one reconciliation lifecycle.
func TestWebhookPolicyReconcileTracksSecretRotationAndDeletion(t *testing.T) {
	ctx := context.Background()
	var remote *harborclient.WebhookPolicy
	var createCount, updateCount, deleteCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const collectionPath = "/api/v2.0/projects/42/webhook/policies"
		const itemPath = collectionPath + "/7"
		switch {
		case r.Method == http.MethodPost && r.URL.Path == collectionPath:
			var desired harborclient.WebhookPolicy
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode webhook create: %v", err)
			}
			createCount++
			desired.ID = 7
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
			var desired harborclient.WebhookPolicy
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode webhook update: %v", err)
			}
			updateCount++
			desired.ID = 7
			remote = &desired
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && r.URL.Path == itemPath:
			deleteCount++
			remote = nil
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	connection, passwordSecret := clusterHarborTestObjects(server.URL)
	project := &harborv1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: testNamespace}, Spec: harborv1alpha1.ProjectSpec{HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef()}}, Status: harborv1alpha1.ProjectStatus{HarborProjectID: 42}}
	targetSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "target-auth", Namespace: testNamespace}, Data: map[string][]byte{"token": []byte("first-token")}}
	enabled := true
	policy := &harborv1alpha1.WebhookPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "notifications", Namespace: testNamespace},
		Spec: harborv1alpha1.WebhookPolicySpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef(), DeletionPolicy: harborv1alpha1.DeletionPolicyDelete},
			ProjectRef:     &harborv1alpha1.ProjectReference{Name: project.Name},
			Description:    "send release events",
			Enabled:        &enabled,
			EventTypes:     []string{"PUSH_ARTIFACT"},
			Targets:        []harborv1alpha1.WebhookTargetSpec{{Type: "http", Address: "https://hooks.example.test", AuthHeaderSecretRef: &harborv1alpha1.SecretReference{Name: targetSecret.Name, Key: "token"}}},
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, passwordSecret, project, targetSecret, policy)
	reconciler := &WebhookPolicyReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: policy.Namespace, Name: policy.Name}}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("create webhook policy: %v", err)
	}
	var stored harborv1alpha1.WebhookPolicy
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get webhook policy after create: %v", err)
	}
	if createCount != 1 || stored.Status.HarborWebhookPolicyID != 7 || stored.Status.TargetsHash == "" || remote == nil || remote.Targets[0].AuthHeader != "first-token" {
		t.Fatalf("webhook create calls/status/remote = %d/%#v/%#v", createCount, stored.Status, remote)
	}
	firstHash := stored.Status.TargetsHash
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op webhook reconcile: %v", err)
	}
	if updateCount != 0 {
		t.Fatalf("no-op webhook reconcile updated Harbor %d times", updateCount)
	}

	var storedSecret corev1.Secret
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: targetSecret.Namespace, Name: targetSecret.Name}, &storedSecret); err != nil {
		t.Fatalf("get webhook target Secret: %v", err)
	}
	storedSecret.Data["token"] = []byte("rotated-token")
	if err := kubeClient.Update(ctx, &storedSecret); err != nil {
		t.Fatalf("rotate webhook Secret: %v", err)
	}
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: targetSecret.Namespace, Name: targetSecret.Name}, &storedSecret); err != nil {
		t.Fatal(err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile webhook credential rotation: %v", err)
	}
	if updateCount != 1 || remote.Targets[0].AuthHeader != "rotated-token" {
		t.Fatalf("webhook rotation updates/target = %d/%#v", updateCount, remote.Targets)
	}
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get webhook status after rotation: %v", err)
	}
	rotatedHash := stored.Status.TargetsHash
	if rotatedHash == "" || rotatedHash == firstHash {
		t.Fatalf("webhook target hash was not updated after rotation: %q", rotatedHash)
	}
	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete webhook policy: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("finalize webhook policy deletion: %v", err)
	}
	if deleteCount != 1 || remote != nil {
		t.Fatalf("webhook deletion calls/remote = %d/%#v", deleteCount, remote)
	}
}

func TestRetentionExistingPolicyIDValidationAndResolution(t *testing.T) {
	ctx := context.Background()
	mode := "valid"
	hc := newUnitHarborClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2.0/projects/42" {
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
			return
		}
		retentionID := "123"
		switch mode {
		case "empty":
			retentionID = ""
		case "invalid":
			retentionID = "not-a-number"
		case "missing":
			writeJSON(t, w, harborclient.Project{ProjectID: 42})
			return
		}
		writeJSON(t, w, harborclient.Project{ProjectID: 42, Metadata: harborclient.ProjectMetadata{RetentionID: &retentionID}})
	}))
	reconciler := &RetentionPolicyReconciler{}
	scope := &harborv1alpha1.RetentionScope{Level: retentionProjectScope, Ref: 42}
	if got, err := reconciler.findExistingRetentionID(ctx, hc, scope); err != nil || got != 123 {
		t.Fatalf("retention ID = %d, %v; want 123", got, err)
	}
	for _, test := range []struct{ mode, want string }{{"empty", "no retention_id"}, {"missing", "no retention_id"}, {"invalid", "parse retention_id"}} {
		t.Run(test.mode, func(t *testing.T) {
			mode = test.mode
			if _, err := reconciler.findExistingRetentionID(ctx, hc, scope); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("retention metadata error = %v, want %q", err, test.want)
			}
		})
	}
	if _, err := reconciler.findExistingRetentionID(ctx, hc, nil); err == nil || !strings.Contains(err.Error(), "without project scope") {
		t.Fatalf("missing retention scope error = %v", err)
	}
	if _, err := reconciler.findExistingRetentionID(ctx, hc, &harborv1alpha1.RetentionScope{Level: retentionProjectScope}); err == nil {
		t.Fatal("zero project scope ref was accepted")
	}
}

func TestProjectDeletionGuardOnlyBlocksSameNamespaceDependentPolicies(t *testing.T) {
	ctx := context.Background()
	project := &harborv1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "team-a"}}
	policy := func(name, namespace, projectNamespace string) *harborv1alpha1.PreheatPolicy {
		return &harborv1alpha1.PreheatPolicy{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}, Spec: harborv1alpha1.PreheatPolicySpec{ProjectRef: harborv1alpha1.ProjectReference{Name: "demo", Namespace: projectNamespace}}}
	}
	kubeClient := newAdvancedUnitClient(t, project, policy("other-project", "team-a", "team-b"), policy("other-namespace", "team-b", ""))
	reconciler := &ProjectReconciler{Client: kubeClient}
	if err := reconciler.ensureNoPreheatPolicies(ctx, project); err != nil {
		t.Fatalf("unrelated namespace policy blocked deletion: %v", err)
	}
	dependent := policy("dependent", "team-a", "")
	if err := kubeClient.Create(ctx, dependent); err != nil {
		t.Fatalf("create dependent policy: %v", err)
	}
	if err := reconciler.ensureNoPreheatPolicies(ctx, project); err == nil || !strings.Contains(err.Error(), "still referenced by PreheatPolicy team-a/dependent") {
		t.Fatalf("dependent policy deletion guard error = %v", err)
	}
}

func TestProjectDeletionTreatsAlreadyMissingProjectAsComplete(t *testing.T) {
	ctx := context.Background()
	deleteCount := 0
	hc := newUnitHarborClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v2.0/projects/42" {
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
			return
		}
		deleteCount++
		w.WriteHeader(http.StatusNotFound)
	}))
	reconciler := &ProjectReconciler{}
	cr := &harborv1alpha1.Project{Status: harborv1alpha1.ProjectStatus{HarborProjectID: 42}}
	if err := reconciler.deleteProject(ctx, hc, cr); err != nil || deleteCount != 1 {
		t.Fatalf("delete missing project = %v, requests=%d", err, deleteCount)
	}
	cr.Status.HarborProjectID = 0
	if err := reconciler.deleteProject(ctx, hc, cr); err != nil || deleteCount != 1 {
		t.Fatalf("delete project without ID = %v, requests=%d", err, deleteCount)
	}
}

func TestMemberDeletionHandlesPersistedMissingAndPartialIdentity(t *testing.T) {
	ctx := context.Background()
	deleteCount := 0
	hc := newUnitHarborClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v2.0/projects/42/members/9" {
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
			return
		}
		deleteCount++
		w.WriteHeader(http.StatusNotFound)
	}))
	reconciler := &MemberReconciler{}
	member := &harborv1alpha1.Member{Status: harborv1alpha1.MemberStatus{HarborProjectID: 42, HarborMemberID: 9}}
	if err := reconciler.ensureMemberAbsent(ctx, hc, member); err != nil || deleteCount != 1 {
		t.Fatalf("delete absent member = %v, requests=%d", err, deleteCount)
	}
	member.Status.HarborMemberID = 0
	if err := reconciler.ensureMemberAbsent(ctx, hc, member); err == nil || !strings.Contains(err.Error(), "incomplete Harbor identity") {
		t.Fatalf("partial member identity error = %v", err)
	}
}

func TestUserAndGroupMemberIdentityValidation(t *testing.T) {
	ctx := context.Background()
	reconciler := &MemberReconciler{}
	for _, test := range []struct {
		name  string
		user  *harborv1alpha1.MemberUser
		group *harborv1alpha1.MemberGroup
		want  string
	}{
		{name: "neither", want: "found none"},
		{name: "both", user: &harborv1alpha1.MemberUser{}, group: &harborv1alpha1.MemberGroup{}, want: "found both"},
	} {
		t.Run(test.name, func(t *testing.T) {
			member := &harborv1alpha1.Member{Spec: harborv1alpha1.MemberSpec{MemberUser: test.user, MemberGroup: test.group}}
			if _, _, _, err := reconciler.desiredEntityFromSpec(ctx, member, 2); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("invalid member identity error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestMemberDesiredUserAndGroupEntitiesResolveKubernetesReferences(t *testing.T) {
	ctx := context.Background()
	user := &harborv1alpha1.User{ObjectMeta: metav1.ObjectMeta{Name: "alice", Namespace: testNamespace}}
	claim := &harborv1alpha1.UserGroupClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "developers", Namespace: testNamespace},
		Spec:       harborv1alpha1.UserGroupClaimSpec{GroupName: "developers", GroupType: 2},
		Status:     harborv1alpha1.UserGroupClaimStatus{HarborGroupID: 17},
	}
	meta.SetStatusCondition(&claim.Status.Conditions, metav1.Condition{Type: ConditionReady, Status: metav1.ConditionTrue, Reason: "Ready", Message: "ready", ObservedGeneration: 1})
	kubeClient := newAdvancedUnitClient(t, user, claim)
	reconciler := &MemberReconciler{Client: kubeClient}

	userMember := &harborv1alpha1.Member{ObjectMeta: metav1.ObjectMeta{Name: "alice-in-demo", Namespace: testNamespace}, Spec: harborv1alpha1.MemberSpec{MemberUser: &harborv1alpha1.MemberUser{UserRef: harborv1alpha1.UserReference{Name: user.Name}}}}
	entityType, entityName, payload, err := reconciler.desiredEntityFromSpec(ctx, userMember, 4)
	if err != nil || entityType != "u" || entityName != "alice" || payload.RoleID != 4 || payload.MemberUser == nil || payload.MemberUser.Username != "alice" {
		t.Fatalf("user member entity = %q/%q %#v, %v", entityType, entityName, payload, err)
	}

	groupMember := &harborv1alpha1.Member{ObjectMeta: metav1.ObjectMeta{Name: "developers-in-demo", Namespace: testNamespace}, Spec: harborv1alpha1.MemberSpec{MemberGroup: &harborv1alpha1.MemberGroup{GroupClaimRef: harborv1alpha1.UserGroupClaimReference{Name: claim.Name}}}}
	entityType, entityName, payload, err = reconciler.desiredEntityFromSpec(ctx, groupMember, 2)
	if err != nil || entityType != "g" || entityName != "developers" || payload.RoleID != 2 || payload.MemberGroup == nil || payload.MemberGroup.GroupType != 2 {
		t.Fatalf("group member entity = %q/%q %#v, %v", entityType, entityName, payload, err)
	}
	if _, err := convertRoleNameToID("MAINTAINER"); err != nil {
		t.Fatalf("role names are case-insensitive: %v", err)
	}
	if _, err := convertRoleNameToID("owner"); err == nil {
		t.Fatal("unsupported Harbor role name was accepted")
	}
}

func TestMemberEnsurePresentCreatesAndUpdatesMembershipRole(t *testing.T) {
	ctx := context.Background()
	project := &harborv1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: testNamespace}, Status: harborv1alpha1.ProjectStatus{HarborProjectID: 42}}
	user := &harborv1alpha1.User{ObjectMeta: metav1.ObjectMeta{Name: "alice", Namespace: testNamespace}}
	member := &harborv1alpha1.Member{ObjectMeta: metav1.ObjectMeta{Name: "alice-in-demo", Namespace: testNamespace}, Spec: harborv1alpha1.MemberSpec{ProjectRef: harborv1alpha1.ProjectReference{Name: project.Name}, MemberUser: &harborv1alpha1.MemberUser{UserRef: harborv1alpha1.UserReference{Name: user.Name}}}}
	kubeClient := newAdvancedUnitClient(t, project, user)
	reconciler := &MemberReconciler{Client: kubeClient}

	var createPayload harborclient.CreateMemberRequest
	createServer := newUnitHarborClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/projects/42/members":
			writeJSON(t, w, []harborclient.ProjectMember{})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2.0/projects/42/members":
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Errorf("decode member create request: %v", err)
			}
			w.Header().Set("Location", "/api/v2.0/projects/42/members/91")
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	observed, err := reconciler.ensureMemberPresent(ctx, createServer, member, 3)
	if err != nil || observed != (observedMember{projectID: 42, memberID: 91}) || createPayload.RoleID != 3 || createPayload.MemberUser == nil || createPayload.MemberUser.Username != "alice" {
		t.Fatalf("created member = %#v, payload=%#v, err=%v", observed, createPayload, err)
	}

	member.Spec.CreationPolicy = harborv1alpha1.CreationPolicyAdopt
	var updatedRole int
	updateServer := newUnitHarborClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/projects/42/members":
			writeJSON(t, w, []harborclient.ProjectMember{{ID: 91, ProjectID: 42, EntityName: "ALICE", EntityType: "U", RoleID: 1}})
		case r.Method == http.MethodPut && r.URL.Path == "/api/v2.0/projects/42/members/91":
			var body struct {
				RoleID int `json:"role_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode member role update: %v", err)
			}
			updatedRole = body.RoleID
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	observed, err = reconciler.ensureMemberPresent(ctx, updateServer, member, 4)
	if err != nil || observed != (observedMember{projectID: 42, memberID: 91}) || updatedRole != 4 {
		t.Fatalf("updated member = %#v, role=%d, err=%v", observed, updatedRole, err)
	}
}

func TestMemberEnsureAbsentDiscoversAndRemovesMatchingMemberships(t *testing.T) {
	ctx := context.Background()
	project := &harborv1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: testNamespace}, Status: harborv1alpha1.ProjectStatus{HarborProjectID: 42}}
	user := &harborv1alpha1.User{ObjectMeta: metav1.ObjectMeta{Name: "alice", Namespace: testNamespace}}
	member := &harborv1alpha1.Member{ObjectMeta: metav1.ObjectMeta{Name: "alice-in-demo", Namespace: testNamespace}, Spec: harborv1alpha1.MemberSpec{ProjectRef: harborv1alpha1.ProjectReference{Name: project.Name}, MemberUser: &harborv1alpha1.MemberUser{UserRef: harborv1alpha1.UserReference{Name: user.Name}}}}
	kubeClient := newAdvancedUnitClient(t, project, user)
	var deleted []string
	hc := newUnitHarborClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/projects/42/members":
			writeJSON(t, w, []harborclient.ProjectMember{
				{ID: 1, EntityName: "unrelated", EntityType: "u"},
				{ID: 9, EntityName: "ALICE", EntityType: "U"},
				{ID: 10, EntityName: "alice", EntityType: "u"},
			})
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v2.0/projects/42/members/"):
			deleted = append(deleted, strings.TrimPrefix(r.URL.Path, "/api/v2.0/projects/42/members/"))
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	reconciler := &MemberReconciler{Client: kubeClient}
	if err := reconciler.ensureMemberAbsent(ctx, hc, member); err != nil {
		t.Fatalf("discover and remove memberships: %v", err)
	}
	if len(deleted) != 2 || deleted[0] != "9" || deleted[1] != "10" {
		t.Fatalf("deleted membership IDs = %v, want matching 9 and 10 only", deleted)
	}
}

func TestProjectRequestConversionAndRetentionScopeEdgeCases(t *testing.T) {
	ctx := context.Background()
	project := &harborv1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: testNamespace}, Spec: harborv1alpha1.ProjectSpec{Public: true, Metadata: &harborv1alpha1.ProjectMetadata{Severity: ptrString("high"), ProxySpeedKB: ptrString("512")}, CVEAllowlist: &harborv1alpha1.CVEAllowlist{Items: []harborv1alpha1.CVEAllowlistItem{{CveID: "CVE-2026-1"}}}}}
	reconciler := &ProjectReconciler{}
	req, err := reconciler.buildCreateReq(ctx, project)
	if err != nil || req.ProjectName != "demo" || req.Metadata.Public != "true" || req.Metadata.Severity == nil || *req.Metadata.Severity != "high" || req.CVEAllowlist == nil || len(req.CVEAllowlist.Items) != 1 {
		t.Fatalf("project request conversion = %#v, %v", req, err)
	}

	policy := &harborv1alpha1.RetentionPolicy{ObjectMeta: metav1.ObjectMeta{Name: "retention", Namespace: testNamespace}, Spec: harborv1alpha1.RetentionPolicySpec{ProjectRef: &harborv1alpha1.ProjectReference{Name: "demo"}}}
	kubeClient := newAdvancedUnitClient(t, &harborv1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: testNamespace}, Status: harborv1alpha1.ProjectStatus{HarborProjectID: 42}})
	scope, err := resolveRetentionScope(ctx, OperatorOptions{}, kubeClient, policy)
	if err != nil || scope.Level != retentionProjectScope || scope.Ref != 42 {
		t.Fatalf("resolved retention project scope = %#v, %v", scope, err)
	}
	policy.Spec.Scope = &harborv1alpha1.RetentionScope{Level: retentionProjectScope, Ref: 1}
	if _, err := resolveRetentionScope(ctx, OperatorOptions{}, kubeClient, policy); err == nil || !strings.Contains(err.Error(), "scope.ref must not be set") {
		t.Fatalf("conflicting explicit retention scope error = %v", err)
	}
}

func TestRobotImagePullSecretIsOwnedScopedAndReadable(t *testing.T) {
	ctx := context.Background()
	robot := &harborv1alpha1.Robot{ObjectMeta: metav1.ObjectMeta{Name: "builder", Namespace: testNamespace}, Status: harborv1alpha1.RobotStatus{Username: "robot$team+builder"}}
	kubeClient := newAdvancedUnitClient(t, robot)
	reconciler := &RobotReconciler{Client: kubeClient}
	hc := harborclient.New("https://harbor.example.test:8443", "", "")
	ref := &harborv1alpha1.SecretOutputReference{Name: "builder-pull"}
	if err := reconciler.reconcileImagePullSecret(ctx, hc, robot, ref, "robot-token"); err != nil {
		t.Fatalf("create image-pull Secret: %v", err)
	}
	token, err := reconciler.readImagePullSecretToken(ctx, hc, robot, ref)
	if err != nil || token != "robot-token" {
		t.Fatalf("read image-pull token = %q, %v", token, err)
	}
	var stored corev1.Secret
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: ref.Name}, &stored); err != nil {
		t.Fatalf("get image-pull Secret: %v", err)
	}
	if stored.Type != corev1.SecretTypeDockerConfigJson || !robotSecretOwnedBy(&stored, robot) {
		t.Fatalf("image-pull Secret metadata = type %q, labels=%v annotations=%v", stored.Type, stored.Labels, stored.Annotations)
	}
	if err := reconciler.reconcileImagePullSecret(ctx, hc, robot, ref, ""); err == nil || !strings.Contains(err.Error(), "robot secret is empty") {
		t.Fatalf("empty image-pull token error = %v", err)
	}

	stored.Data[corev1.DockerConfigJsonKey] = []byte("not-json")
	if err := kubeClient.Update(ctx, &stored); err != nil {
		t.Fatalf("corrupt image-pull Secret: %v", err)
	}
	if _, err := reconciler.readImagePullSecretToken(ctx, hc, robot, ref); err == nil || !strings.Contains(err.Error(), "parse image pull Secret") {
		t.Fatalf("malformed image-pull Secret error = %v", err)
	}

	otherOwner := &harborv1alpha1.Robot{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: testNamespace}}
	foreign := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "foreign-pull", Namespace: testNamespace}, Type: corev1.SecretTypeDockerConfigJson, Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{"auths":{}}`)}}
	if err := kubeClient.Create(ctx, foreign); err != nil {
		t.Fatalf("create foreign image-pull Secret: %v", err)
	}
	_ = otherOwner
	if _, err := reconciler.readImagePullSecretToken(ctx, hc, robot, &harborv1alpha1.SecretOutputReference{Name: foreign.Name}); err == nil || !strings.Contains(err.Error(), "not managed by Robot") {
		t.Fatalf("foreign image-pull Secret error = %v", err)
	}
	allowCrossNamespace := false
	reconciler.Options = OperatorOptions{allowCrossNamespaceReferences: &allowCrossNamespace}
	if err := reconciler.reconcileImagePullSecret(ctx, hc, robot, &harborv1alpha1.SecretOutputReference{Name: "cross", Namespace: "other"}, "token"); err == nil || !strings.Contains(err.Error(), "cross-namespace") {
		t.Fatalf("disallowed cross-namespace image-pull Secret error = %v", err)
	}
}

func TestRobotAdoptionMatchesCanonicalUsernamesAndDeletionIsIdempotent(t *testing.T) {
	ctx := context.Background()
	var listCalls, deleteCalls int
	hc := newUnitHarborClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/robots":
			listCalls++
			if r.URL.Query().Get("q") != "" {
				writeJSON(t, w, []harborclient.Robot{})
				return
			}
			writeJSON(t, w, []harborclient.Robot{{ID: 52, Name: "robot$team+builder", Level: "SYSTEM"}})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v2.0/robots/52":
			deleteCalls++
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	robot := &harborv1alpha1.Robot{ObjectMeta: metav1.ObjectMeta{Name: "builder", Namespace: testNamespace}, Spec: harborv1alpha1.RobotSpec{Level: "system"}}
	kubeClient := newAdvancedUnitClient(t, robot)
	reconciler := &RobotReconciler{Client: kubeClient}
	adopted, err := reconciler.adoptExisting(ctx, hc, robot)
	if err != nil || !adopted || robot.Status.HarborRobotID != 52 || listCalls != 2 {
		t.Fatalf("robot adoption = %t, ID=%d, lists=%d, err=%v", adopted, robot.Status.HarborRobotID, listCalls, err)
	}
	if err := reconciler.deleteRobot(ctx, hc, robot); err != nil || deleteCalls != 1 {
		t.Fatalf("delete already missing robot = %v, requests=%d", err, deleteCalls)
	}
	robot.Status.HarborRobotID = 0
	if err := reconciler.deleteRobot(ctx, hc, robot); err != nil || deleteCalls != 1 {
		t.Fatalf("delete robot without ID = %v, requests=%d", err, deleteCalls)
	}
	for _, test := range []struct {
		desired, current string
		want             bool
	}{
		{"builder", "BUILDER", true},
		{"builder", "robot$builder", true},
		{"builder", "robot+builder", true},
		{"builder", "other", false},
	} {
		if got := robotNameMatches(test.desired, test.current); got != test.want {
			t.Errorf("robotNameMatches(%q,%q)=%t, want %t", test.desired, test.current, got, test.want)
		}
	}
}

func TestPreheatDependencyErrorWaitsForMissingDependencyAndReportsOtherErrors(t *testing.T) {
	ctx := context.Background()
	policy := &harborv1alpha1.PreheatPolicy{ObjectMeta: metav1.ObjectMeta{Name: "warming", Namespace: testNamespace}}
	kubeClient := newAdvancedUnitClient(t, policy)
	reconciler := &PreheatPolicyReconciler{Client: kubeClient}
	result, err := reconciler.dependencyError(ctx, policy, &dependencyNotReadyError{kind: "Project", key: types.NamespacedName{Namespace: testNamespace, Name: "missing"}})
	if err != nil || result.RequeueAfter != dependencyRequeueInterval {
		t.Fatalf("dependency retry = %#v, %v", result, err)
	}
	var stored harborv1alpha1.PreheatPolicy
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: policy.Name}, &stored); err != nil {
		t.Fatalf("get dependency status: %v", err)
	}
	condition := findCondition(stored.Status.Conditions, ConditionReady)
	if condition == nil || condition.Reason != "DependencyNotReady" {
		t.Fatalf("dependency Ready condition = %#v", condition)
	}

	policy.Status.Conditions = nil
	if _, err := reconciler.dependencyError(ctx, policy, fmt.Errorf("remote dependency API failed")); err == nil {
		t.Fatal("operational dependency failure was treated as a retryable missing dependency")
	}
}

func TestPreheatProjectAndProviderDependenciesRequireReadySameInstanceResources(t *testing.T) {
	ctx := context.Background()
	primary := &harborv1alpha1.ClusterHarborConnection{ObjectMeta: metav1.ObjectMeta{Name: "primary"}, Spec: harborv1alpha1.HarborConnectionSpec{BaseURL: "https://harbor-primary.example.test"}}
	other := &harborv1alpha1.ClusterHarborConnection{ObjectMeta: metav1.ObjectMeta{Name: "other"}, Spec: harborv1alpha1.HarborConnectionSpec{BaseURL: "https://harbor-other.example.test"}}
	connectionRef := func(name string) *harborv1alpha1.HarborConnectionReference {
		return &harborv1alpha1.HarborConnectionReference{Name: name, Kind: harborv1alpha1.HarborConnectionReferenceKindCluster}
	}
	project := &harborv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: testNamespace},
		Spec:       harborv1alpha1.ProjectSpec{HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: connectionRef(other.Name)}},
		Status: harborv1alpha1.ProjectStatus{HarborProjectID: 42, HarborStatusBase: harborv1alpha1.HarborStatusBase{
			ResolvedHarborConnection: &harborv1alpha1.HarborConnectionBinding{Kind: harborv1alpha1.HarborConnectionReferenceKindCluster, Name: other.Name},
		}},
	}
	provider := &harborv1alpha1.PreheatInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "dragonfly"},
		Spec:       harborv1alpha1.PreheatInstanceSpec{HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: connectionRef(primary.Name)}},
		Status: harborv1alpha1.PreheatInstanceStatus{HarborInstanceID: 7, HarborStatusBase: harborv1alpha1.HarborStatusBase{
			ResolvedHarborConnection: &harborv1alpha1.HarborConnectionBinding{Kind: harborv1alpha1.HarborConnectionReferenceKindCluster, Name: primary.Name},
		}},
	}
	policy := &harborv1alpha1.PreheatPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "warm", Namespace: testNamespace},
		Spec: harborv1alpha1.PreheatPolicySpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: connectionRef(primary.Name)},
			ProjectRef:     harborv1alpha1.ProjectReference{Name: project.Name},
			ProviderRef:    harborv1alpha1.PreheatInstanceReference{Name: provider.Name},
		},
	}
	kubeClient := newAdvancedUnitClient(t, primary, other, project, provider)
	reconciler := &PreheatPolicyReconciler{Client: kubeClient}
	if _, err := reconciler.resolveProject(ctx, policy); err == nil || !strings.Contains(err.Error(), "different Harbor instances") {
		t.Fatalf("cross-instance project dependency error = %v", err)
	}
	project.Spec.HarborConnectionRef = connectionRef(primary.Name)
	if err := kubeClient.Update(ctx, project); err != nil {
		t.Fatalf("update project connection: %v", err)
	}
	project.Status.ResolvedHarborConnection.Name = primary.Name
	if err := kubeClient.Status().Update(ctx, project); err != nil {
		t.Fatalf("update project connection status: %v", err)
	}
	if id, err := reconciler.resolveProject(ctx, policy); err != nil || id != 42 {
		t.Fatalf("ready project dependency = %d, %v", id, err)
	}
	if got, err := reconciler.resolveProvider(ctx, policy, harborclient.New("https://harbor-primary.example.test/", "", "")); err != nil || got == nil || got.Status.HarborInstanceID != 7 {
		t.Fatalf("ready provider dependency = %#v, %v", got, err)
	}

	policy.Spec.ProviderRef.Name = "missing"
	if _, err := reconciler.resolveProvider(ctx, policy, harborclient.New("https://harbor-primary.example.test", "", "")); !isDependencyNotReady(err) {
		t.Fatalf("missing provider error = %v, want dependency-not-ready", err)
	}
	policy.Spec.ProviderRef.Name = ""
	if _, err := reconciler.resolveProvider(ctx, policy, harborclient.New("https://harbor-primary.example.test", "", "")); err == nil || !strings.Contains(err.Error(), "providerRef is required") {
		t.Fatalf("empty provider reference error = %v", err)
	}
	provider.Status.HarborInstanceID = 0
	if err := kubeClient.Status().Update(ctx, provider); err != nil {
		t.Fatalf("clear provider readiness: %v", err)
	}
	policy.Spec.ProviderRef.Name = provider.Name
	if _, err := reconciler.resolveProvider(ctx, policy, harborclient.New("https://harbor-primary.example.test", "", "")); !isDependencyNotReady(err) {
		t.Fatalf("unready provider error = %v, want dependency-not-ready", err)
	}
	policy.Spec.ProviderRef.Name = provider.Name
	provider.Spec.HarborConnectionRef = connectionRef(other.Name)
	if err := kubeClient.Update(ctx, provider); err != nil {
		t.Fatalf("change provider connection: %v", err)
	}
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: provider.Name}, provider); err != nil {
		t.Fatalf("reload provider after connection update: %v", err)
	}
	provider.Status.HarborInstanceID = 7
	if err := kubeClient.Status().Update(ctx, provider); err != nil {
		t.Fatalf("restore provider readiness: %v", err)
	}
	if _, err := reconciler.resolveProvider(ctx, policy, harborclient.New("https://harbor-primary.example.test", "", "")); err == nil || !strings.Contains(err.Error(), "different Harbor instances") {
		t.Fatalf("cross-instance provider error = %v", err)
	}
}

func TestFindUserGroupRequiresCompatibleIdentity(t *testing.T) {
	ctx := context.Background()
	hc := newUnitHarborClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2.0/usergroups" {
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
			return
		}
		writeJSON(t, w, []harborclient.UserGroup{{ID: 19, GroupName: "Developers", GroupType: 2, LDAPGroupDN: "cn=dev,dc=example"}})
	}))
	desired := harborclient.UserGroup{GroupName: "developers", GroupType: 2, LDAPGroupDN: "CN=DEV,DC=EXAMPLE"}
	group, found, err := findUserGroup(ctx, hc, desired)
	if err != nil || !found || group == nil || group.ID != 19 {
		t.Fatalf("compatible UserGroup lookup = %#v, found=%t, err=%v", group, found, err)
	}
	desired.GroupType = 1
	if _, found, err := findUserGroup(ctx, hc, desired); found || err == nil || !strings.Contains(err.Error(), "incompatible type or LDAP group DN") {
		t.Fatalf("incompatible UserGroup lookup = found %t, err %v", found, err)
	}
	desired.GroupName = "other"
	if group, found, err := findUserGroup(ctx, hc, desired); err != nil || found || group != nil {
		t.Fatalf("missing UserGroup lookup = %#v, found=%t, err=%v", group, found, err)
	}
}

func TestReplicationPolicyAdoptionPersistsMatchingPolicy(t *testing.T) {
	ctx := context.Background()
	var query string
	hc := newUnitHarborClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2.0/replication/policies" {
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
			return
		}
		query = r.URL.Query().Get("name")
		writeJSON(t, w, []harborclient.ReplicationPolicy{{ID: 28, Name: "Mirror"}})
	}))
	cr := &harborv1alpha1.ReplicationPolicy{ObjectMeta: metav1.ObjectMeta{Name: "mirror", Namespace: testNamespace}}
	kubeClient := newAdvancedUnitClient(t, cr)
	reconciler := &ReplicationPolicyReconciler{Client: kubeClient}
	adopted, err := reconciler.adoptExisting(ctx, hc, cr)
	if err != nil || !adopted || cr.Status.HarborReplicationPolicyID != 28 || query != "mirror" {
		t.Fatalf("replication adoption = %t, ID=%d, query=%q, err=%v", adopted, cr.Status.HarborReplicationPolicyID, query, err)
	}
}

func TestClusterHarborConnectionReconcilesReachabilityAndAuthentication(t *testing.T) {
	ctx := context.Background()
	var authChecks int
	var pingFails, rejectAuth bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2.0/ping":
			if pingFails {
				http.Error(w, "unavailable", http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
		case "/api/v2.0/users/current/permissions":
			user, password, ok := r.BasicAuth()
			if rejectAuth || !ok || user != "admin" || password != "test-password" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			authChecks++
			writeJSON(t, w, []any{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	unauthenticated := &harborv1alpha1.ClusterHarborConnection{ObjectMeta: metav1.ObjectMeta{Name: "public"}, Spec: harborv1alpha1.HarborConnectionSpec{BaseURL: server.URL}}
	client := newAdvancedUnitClient(t, unauthenticated)
	reconciler := &ClusterHarborConnectionReconciler{Client: client}
	if _, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: unauthenticated.Name}}); err != nil {
		t.Fatalf("unauthenticated connection reconcile: %v", err)
	}
	var stored harborv1alpha1.ClusterHarborConnection
	if err := client.Get(ctx, types.NamespacedName{Name: unauthenticated.Name}, &stored); err != nil {
		t.Fatalf("get unauthenticated connection: %v", err)
	}
	if stored.Status.Authenticated {
		t.Fatal("connection without credentials was marked authenticated")
	}
	assertReadyCondition(t, stored.Status.Conditions)

	connection, secret := clusterHarborTestObjects(server.URL)
	client = newAdvancedUnitClient(t, connection, secret)
	reconciler.Client = client
	if _, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: connection.Name}}); err != nil {
		t.Fatalf("authenticated connection reconcile: %v", err)
	}
	if err := client.Get(ctx, types.NamespacedName{Name: connection.Name}, &stored); err != nil {
		t.Fatalf("get authenticated connection: %v", err)
	}
	if !stored.Status.Authenticated || authChecks != 1 {
		t.Fatalf("authenticated status/calls = %t/%d", stored.Status.Authenticated, authChecks)
	}
	assertReadyCondition(t, stored.Status.Conditions)

	pingFails = true
	unreachable := &harborv1alpha1.ClusterHarborConnection{ObjectMeta: metav1.ObjectMeta{Name: "unreachable"}, Spec: harborv1alpha1.HarborConnectionSpec{BaseURL: server.URL}}
	client = newAdvancedUnitClient(t, unreachable)
	reconciler.Client = client
	if _, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: unreachable.Name}}); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("Harbor ping failure = %v, want 503", err)
	}
	if err := client.Get(ctx, types.NamespacedName{Name: unreachable.Name}, &stored); err != nil {
		t.Fatalf("get unreachable connection status: %v", err)
	}
	condition := meta.FindStatusCondition(stored.Status.Conditions, ConditionReady)
	if condition == nil || condition.Status != metav1.ConditionFalse {
		t.Fatalf("unreachable Harbor connection Ready condition = %#v", condition)
	}

	pingFails = false
	rejectAuth = true
	unauthorized := connection.DeepCopy()
	unauthorized.Name = "unauthorized"
	client = newAdvancedUnitClient(t, unauthorized, secret)
	reconciler.Client = client
	if _, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: unauthorized.Name}}); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("Harbor authentication failure = %v, want 401", err)
	}
	if err := client.Get(ctx, types.NamespacedName{Name: unauthorized.Name}, &stored); err != nil {
		t.Fatalf("get unauthorized connection status: %v", err)
	}
	condition = meta.FindStatusCondition(stored.Status.Conditions, ConditionReady)
	if condition == nil || condition.Status != metav1.ConditionFalse {
		t.Fatalf("unauthorized Harbor connection Ready condition = %#v", condition)
	}

	bad := &harborv1alpha1.ClusterHarborConnection{ObjectMeta: metav1.ObjectMeta{Name: "bad"}, Spec: harborv1alpha1.HarborConnectionSpec{BaseURL: "bad-url"}}
	client = newAdvancedUnitClient(t, bad)
	reconciler.Client = client
	if _, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: bad.Name}}); err == nil {
		t.Fatal("invalid Harbor URL did not surface an error")
	}
}

func TestNamespacedHarborConnectionAuthenticatesAndReportsCredentialErrors(t *testing.T) {
	ctx := context.Background()
	authenticatedCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2.0/ping":
			w.WriteHeader(http.StatusOK)
		case "/api/v2.0/users/current/permissions":
			user, password, ok := r.BasicAuth()
			if !ok || user != "service" || password != "connection-token" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			authenticatedCalls++
			writeJSON(t, w, []any{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "connection-password", Namespace: testNamespace}, Data: map[string][]byte{"password": []byte("connection-token")}}
	connection := &harborv1alpha1.HarborConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "harbor", Namespace: testNamespace},
		Spec: harborv1alpha1.HarborConnectionSpec{
			BaseURL:     server.URL,
			Credentials: &harborv1alpha1.Credentials{Type: "basic", Username: "service", PasswordSecretRef: harborv1alpha1.SecretReference{Name: secret.Name, Key: "password"}},
		},
	}
	kubeClient := newAdvancedUnitClient(t, secret, connection)
	reconciler := &HarborConnectionReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: connection.Namespace, Name: connection.Name}}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("authenticated HarborConnection reconcile: %v", err)
	}
	var stored harborv1alpha1.HarborConnection
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get authenticated HarborConnection: %v", err)
	}
	if !stored.Status.Authenticated || authenticatedCalls != 1 {
		t.Fatalf("authenticated status/requests = %t/%d", stored.Status.Authenticated, authenticatedCalls)
	}
	assertReadyCondition(t, stored.Status.Conditions)

	broken := &harborv1alpha1.HarborConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "broken", Namespace: testNamespace},
		Spec: harborv1alpha1.HarborConnectionSpec{
			BaseURL:     server.URL,
			Credentials: &harborv1alpha1.Credentials{Type: "basic", Username: "service", PasswordSecretRef: harborv1alpha1.SecretReference{Name: "missing", Key: "password"}},
		},
	}
	brokenClient := newAdvancedUnitClient(t, broken)
	reconciler.Client = brokenClient
	brokenReq := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: broken.Namespace, Name: broken.Name}}
	if _, err := reconciler.Reconcile(ctx, brokenReq); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing Harbor credentials error = %v", err)
	}
	if err := brokenClient.Get(ctx, brokenReq.NamespacedName, &stored); err != nil {
		t.Fatalf("get broken HarborConnection status: %v", err)
	}
	condition := meta.FindStatusCondition(stored.Status.Conditions, ConditionReady)
	if condition == nil || condition.Status != metav1.ConditionFalse {
		t.Fatalf("broken HarborConnection Ready condition = %#v", condition)
	}
}

//nolint:gocyclo // The mock covers HarborRole payload conversion and HTTP lifecycle contracts together.
func TestHarborRoleConstructionAndHarborOperations(t *testing.T) {
	ctx := context.Background()
	var updated harborclient.HarborRole
	server := newUnitHarborClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/roles":
			writeJSON(t, w, []harborclient.HarborRole{
				{ID: 1, Name: "developer", IsBuiltin: true},
				{ID: 12, Name: "ArtifactReader"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2.0/roles":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":21}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/roles/21":
			writeJSON(t, w, harborclient.HarborRole{ID: 21, Name: "ArtifactReader"})
		case r.Method == http.MethodPut && r.URL.Path == "/api/v2.0/roles/21":
			if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
				t.Errorf("decode Harbor role update: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v2.0/roles/21":
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected Harbor request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	reconciler := NewHarborRoleReconciler(newAdvancedUnitClient(t), nil, OperatorOptions{})
	role := &harborv1alpha1.HarborRole{ObjectMeta: metav1.ObjectMeta{Name: "artifact-reader"}, Spec: harborv1alpha1.HarborRoleSpec{
		Description: "read artifacts",
		Permissions: []harborv1alpha1.HarborRolePermission{{Kind: "project", Namespace: "*", Access: []harborv1alpha1.HarborRoleAccess{{Resource: "artifact", Action: "read", Effect: "allow"}}}},
	}}
	desiredValue, err := reconciler.Desired(ctx, server, role)
	if err != nil {
		t.Fatalf("build Harbor role desired state: %v", err)
	}
	desired := desiredValue.(*harborclient.HarborRole)
	if desired.Name != role.Name || desired.Description != role.Spec.Description || len(desired.Permissions) != 1 || desired.Permissions[0].Access[0].Action != "read" {
		t.Fatalf("converted Harbor role = %#v", desired)
	}
	if id, found, builtin, err := reconciler.ops.find(ctx, server, "ARTIFACTREADER"); err != nil || id != 12 || !found || builtin {
		t.Fatalf("find custom Harbor role = %d/%t/%t, %v", id, found, builtin, err)
	}
	if id, found, builtin, err := reconciler.ops.find(ctx, server, "developer"); err != nil || id != 1 || !found || !builtin {
		t.Fatalf("find built-in Harbor role = %d/%t/%t, %v", id, found, builtin, err)
	}
	if id, err := reconciler.ops.create(ctx, server, desired); err != nil || id != 21 {
		t.Fatalf("create Harbor role = %d, %v", id, err)
	}
	currentValue, err := reconciler.ops.get(ctx, server, "ignored", 21)
	if err != nil || currentValue.(*harborclient.HarborRole).ID != 21 {
		t.Fatalf("get Harbor role = %#v, %v", currentValue, err)
	}
	changed := *desired
	changed.Description = "read artifacts and metadata"
	if err := reconciler.ops.update(ctx, server, "ignored", 21, &changed); err != nil || updated.ID != 21 || updated.Description != changed.Description {
		t.Fatalf("update Harbor role = %#v, %v", updated, err)
	}
	if reconciler.ops.equivalent(desired, &changed) || !reconciler.ops.equivalent(desired, desired) || !reconciler.ops.builtin(&harborclient.HarborRole{IsBuiltin: true}) {
		t.Fatal("Harbor role comparison or built-in classification is incorrect")
	}
	if err := reconciler.ops.delete(ctx, server, "ignored", 21); err != nil {
		t.Fatalf("delete already-missing Harbor role: %v", err)
	}
	if !reconciler.SetID(role, 21) || reconciler.SetID(role, 21) || reconciler.ID(role) != 21 {
		t.Fatal("Harbor role status identity setter did not report only actual changes")
	}
}

func TestGlobalHarborEntityIdentityOwnerIgnoresDistinctClusterIdentities(t *testing.T) {
	ctx := context.Background()
	connection := &harborv1alpha1.ClusterHarborConnection{ObjectMeta: metav1.ObjectMeta{Name: "primary"}, Spec: harborv1alpha1.HarborConnectionSpec{BaseURL: "https://harbor.example.test"}}
	current := &harborv1alpha1.HarborRole{ObjectMeta: metav1.ObjectMeta{Name: "custom-role"}}
	peer := &harborv1alpha1.HarborRole{ObjectMeta: metav1.ObjectMeta{Name: "another-role"}}
	kubeClient := newAdvancedUnitClient(t, connection, current, peer)
	reconciler := NewHarborRoleReconciler(kubeClient, nil, OperatorOptions{forcedHarborConnection: connection.Name})
	owner, conflict, err := reconciler.globalIdentityOwner(ctx, current)
	if err != nil || !owner || conflict {
		t.Fatalf("distinct cluster identity ownership = owner %t, conflict %t, err %v", owner, conflict, err)
	}
}

func TestRobotRotationPredicateUsesExpiryBoundary(t *testing.T) {
	robot := &harborv1alpha1.Robot{}
	if shouldRotateRobot(robot) {
		t.Fatal("robot without known expiration was rotated")
	}
	expired := metav1.NewTime(time.Now().Add(-time.Minute))
	robot.Status.ExpiresAt = &expired
	if !shouldRotateRobot(robot) {
		t.Fatal("expired robot was not marked for rotation")
	}
	future := metav1.NewTime(time.Now().Add(time.Hour))
	robot.Status.ExpiresAt = &future
	if shouldRotateRobot(robot) {
		t.Fatal("robot with future expiration was marked for rotation")
	}
}

func ptrString(value string) *string { return &value }
