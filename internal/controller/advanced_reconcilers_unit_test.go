package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

func TestCVEAllowlistSpecConversionAndComparison(t *testing.T) {
	expiresAt := metav1.NewTime(time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC))
	desired := cveAllowlistFromSpec(harborv1alpha1.HarborCVEAllowlistSpec{
		Items:     []harborv1alpha1.HarborCVEAllowlistItem{{CVEID: "CVE-2026-2"}, {CVEID: "CVE-2026-1"}},
		ExpiresAt: &expiresAt,
	})
	if got, want := desired.Items, []harborclient.SystemCVEAllowlistItem{{CVEID: "CVE-2026-1"}, {CVEID: "CVE-2026-2"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("allowlist items = %#v, want sorted %#v", got, want)
	}
	if desired.ExpiresAt == nil || *desired.ExpiresAt != expiresAt.Unix() {
		t.Fatalf("expiresAt = %v, want Unix timestamp %d", desired.ExpiresAt, expiresAt.Unix())
	}

	current := harborclient.SystemCVEAllowlist{
		Items:     []harborclient.SystemCVEAllowlistItem{{CVEID: "CVE-2026-1"}, {CVEID: "CVE-2026-2"}},
		ExpiresAt: desired.ExpiresAt,
	}
	if !sameCVEAllowlist(desired, current) {
		t.Fatal("sameCVEAllowlist() reported drift for items in a different order")
	}
	current.Items = current.Items[:1]
	if sameCVEAllowlist(desired, current) {
		t.Fatal("sameCVEAllowlist() ignored a missing CVE")
	}
	current = harborclient.SystemCVEAllowlist{Items: desired.Items, ExpiresAt: ptrInt64(expiresAt.Unix() + 1)}
	if sameCVEAllowlist(desired, current) {
		t.Fatal("sameCVEAllowlist() ignored expiration drift")
	}
	if !sameCVEAllowlist(harborclient.SystemCVEAllowlist{}, harborclient.SystemCVEAllowlist{Items: []harborclient.SystemCVEAllowlistItem{}}) {
		t.Fatal("sameCVEAllowlist() should treat nil and empty item slices as equivalent")
	}
}

func TestHarborCVEAllowlistReconcileCreateNoopAndDelete(t *testing.T) {
	ctx := context.Background()
	var current = harborclient.SystemCVEAllowlist{Items: []harborclient.SystemCVEAllowlistItem{}, UpdateTime: "2026-01-01T00:00:00Z"}
	var updates []harborclient.SystemCVEAllowlist
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2.0/system/CVEAllowlist" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			writeJSON(t, w, current)
		case http.MethodPut:
			var got harborclient.SystemCVEAllowlist
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode CVE allowlist update: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			updates = append(updates, got)
			current = got
			current.UpdateTime = "2026-01-02T03:04:05Z"
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	connection, secret := clusterHarborTestObjects(server.URL)
	expiresAt := metav1.NewTime(time.Date(2026, time.February, 3, 4, 5, 6, 0, time.UTC))
	allowlist := &harborv1alpha1.HarborCVEAllowlist{
		ObjectMeta: metav1.ObjectMeta{Name: "allowlist"},
		Spec: harborv1alpha1.HarborCVEAllowlistSpec{
			HarborConnectionRef: clusterHarborRef(),
			DeletionPolicy:      harborv1alpha1.DeletionPolicyDelete,
			Items: []harborv1alpha1.HarborCVEAllowlistItem{
				{CVEID: "CVE-2026-2"}, {CVEID: "CVE-2026-1"},
			},
			ExpiresAt: &expiresAt,
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, allowlist)
	reconciler := &HarborCVEAllowlistReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: allowlist.Name}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("Harbor updates = %d, want 1", len(updates))
	}
	if got, want := updates[0].Items, []harborclient.SystemCVEAllowlistItem{{CVEID: "CVE-2026-1"}, {CVEID: "CVE-2026-2"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Harbor update items = %#v, want %#v", got, want)
	}
	if updates[0].ExpiresAt == nil || *updates[0].ExpiresAt != expiresAt.Unix() {
		t.Fatalf("Harbor update expiration = %v, want %d", updates[0].ExpiresAt, expiresAt.Unix())
	}
	var stored harborv1alpha1.HarborCVEAllowlist
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get allowlist after reconcile: %v", err)
	}
	if stored.Status.HarborUpdatedAt == nil || !stored.Status.HarborUpdatedAt.Equal(&metav1.Time{Time: time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)}) {
		t.Fatalf("HarborUpdatedAt = %v, want parsed Harbor timestamp", stored.Status.HarborUpdatedAt)
	}
	assertReadyCondition(t, stored.Status.Conditions)

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op reconcile: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("no-op reconcile sent another Harbor update; updates = %d", len(updates))
	}

	current.UpdateTime = "not-a-timestamp"
	if _, err := reconciler.Reconcile(ctx, req); err == nil || !strings.Contains(err.Error(), "parse Harbor CVE allowlist update time") {
		t.Fatalf("invalid Harbor CVE allowlist update time error = %v", err)
	}
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get allowlist after invalid timestamp: %v", err)
	}
	condition := meta.FindStatusCondition(stored.Status.Conditions, ConditionReady)
	if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "ReconcileError" {
		t.Fatalf("invalid timestamp Ready condition = %#v", condition)
	}
	current.UpdateTime = "2026-01-02T03:04:05Z"
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("recover allowlist after invalid timestamp: %v", err)
	}
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get allowlist after timestamp recovery: %v", err)
	}
	assertReadyCondition(t, stored.Status.Conditions)

	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete allowlist: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile deletion: %v", err)
	}
	if len(updates) != 2 || len(updates[1].Items) != 0 || updates[1].ExpiresAt != nil {
		t.Fatalf("delete update = %#v, want an empty Harbor allowlist", updates)
	}
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); !apierrors.IsNotFound(err) {
		t.Fatalf("allowlist after finalization error = %v, want NotFound", err)
	}
}

//nolint:gocyclo // This test verifies each transition in one observable resource lifecycle.
func TestHarborUserGroupReconcileCreateNoopUpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	var remote *harborclient.UserGroup
	var createCount, updateCount, deleteCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/usergroups":
			groups := []harborclient.UserGroup{}
			if remote != nil {
				groups = append(groups, *remote)
			}
			writeJSON(t, w, groups)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2.0/usergroups":
			var got harborclient.UserGroup
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode create user group: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			createCount++
			got.ID = 42
			remote = &got
			w.Header().Set("Location", "/api/v2.0/usergroups/42")
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/usergroups/42":
			if remote == nil {
				http.NotFound(w, r)
				return
			}
			writeJSON(t, w, remote)
		case r.Method == http.MethodPut && r.URL.Path == "/api/v2.0/usergroups/42":
			var got harborclient.UserGroup
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode user group update: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			updateCount++
			remote = &got
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v2.0/usergroups/42":
			deleteCount++
			remote = nil
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	connection, secret := clusterHarborTestObjects(server.URL)
	group := &harborv1alpha1.HarborUserGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "developers"},
		Spec: harborv1alpha1.HarborUserGroupSpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef()},
			GroupType:      2,
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, group)
	reconciler := NewHarborUserGroupReconciler(kubeClient, kubeClient.Scheme(), OperatorOptions{})
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: group.Name}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}
	var stored harborv1alpha1.HarborUserGroup
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get group after create: %v", err)
	}
	if stored.Status.HarborUserGroupID != 42 || createCount != 1 {
		t.Fatalf("created group ID/calls = %d/%d, want 42/1", stored.Status.HarborUserGroupID, createCount)
	}
	assertReadyCondition(t, stored.Status.Conditions)

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op reconcile: %v", err)
	}
	if updateCount != 0 {
		t.Fatalf("no-op reconcile sent %d updates, want 0", updateCount)
	}

	stored.Spec.LDAPGroupDN = "cn=developers,ou=groups,dc=example,dc=com"
	if err := kubeClient.Update(ctx, &stored); err != nil {
		t.Fatalf("update group spec: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile group update: %v", err)
	}
	if updateCount != 1 || remote == nil || remote.LDAPGroupDN != stored.Spec.LDAPGroupDN {
		t.Fatalf("Harbor group after update = %#v with %d updates", remote, updateCount)
	}

	claim := &harborv1alpha1.UserGroupClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "developers-claim", Namespace: testNamespace},
		Spec: harborv1alpha1.UserGroupClaimSpec{
			HarborClaimSpecBase: harborv1alpha1.HarborClaimSpecBase{HarborConnectionRef: clusterHarborRef()},
			GroupName:           group.Name,
			GroupType:           2,
		},
	}
	if err := kubeClient.Create(ctx, claim); err != nil {
		t.Fatalf("create dependent group claim: %v", err)
	}
	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete group: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err == nil || !strings.Contains(err.Error(), "still claimed by UserGroupClaim") {
		t.Fatalf("deleting a claimed group returned error = %v, want claim dependency error", err)
	}
	if deleteCount != 0 || remote == nil {
		t.Fatalf("claimed group was deleted from Harbor: calls=%d remote=%#v", deleteCount, remote)
	}
	if err := kubeClient.Delete(ctx, claim); err != nil {
		t.Fatalf("delete dependent group claim: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile group deletion after claim removal: %v", err)
	}
	if deleteCount != 1 || remote != nil {
		t.Fatalf("Harbor delete calls/state = %d/%#v, want 1/nil", deleteCount, remote)
	}
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); !apierrors.IsNotFound(err) {
		t.Fatalf("group after finalization error = %v, want NotFound", err)
	}
}

//nolint:gocyclo // This test verifies rotation and redaction across one Harbor-backed lifecycle.
func TestPreheatInstanceReconcileSecretRotationAndStatusRedaction(t *testing.T) {
	ctx := context.Background()
	var remote *harborclient.PreheatInstance
	var received []map[string]string
	var failUpdate bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const listPath = "/api/v2.0/p2p/preheat/instances"
		const itemPath = listPath + "/dragonfly"
		switch {
		case r.Method == http.MethodGet && r.URL.Path == listPath:
			instances := []harborclient.PreheatInstance{}
			if remote != nil {
				instances = append(instances, *remote)
			}
			writeJSON(t, w, instances)
		case r.Method == http.MethodPost && r.URL.Path == listPath:
			var got harborclient.PreheatInstance
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode preheat instance create: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			received = append(received, cloneStringMap(got.AuthInfo))
			got.ID = 55
			got.Status = "healthy"
			remote = &got
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == itemPath:
			if remote == nil {
				http.NotFound(w, r)
				return
			}
			writeJSON(t, w, remote)
		case r.Method == http.MethodPut && r.URL.Path == itemPath:
			var got harborclient.PreheatInstance
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode preheat instance update: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			received = append(received, cloneStringMap(got.AuthInfo))
			if failUpdate {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("credential rejected: " + got.AuthInfo["password"]))
				return
			}
			got.ID = 55
			got.Status = "healthy"
			remote = &got
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	connection, harborSecret := clusterHarborTestObjects(server.URL)
	authSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "preheat-auth", Namespace: testNamespace},
		Data:       map[string][]byte{"username": []byte("cache-user"), "password": []byte("initial-secret")},
	}
	instance := &harborv1alpha1.PreheatInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "dragonfly"},
		Spec: harborv1alpha1.PreheatInstanceSpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef()},
			Vendor:         "dragonfly",
			Endpoint:       "https://cache.example.test",
			AuthMode:       "BASIC",
			AuthInfoSecretRef: &harborv1alpha1.SecretObjectReference{
				Name: "preheat-auth", Namespace: testNamespace,
			},
			Enabled: true,
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, harborSecret, authSecret, instance)
	reconciler := NewPreheatInstanceReconciler(kubeClient, kubeClient.Scheme(), OperatorOptions{})
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: instance.Name}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}
	// Creation is followed by one update because the initial hash is not in
	// status yet; later stable reconciles must not write again.
	if len(received) != 2 || received[0]["username"] != "cache-user" || received[0]["password"] != "initial-secret" || received[1]["password"] != "initial-secret" {
		t.Fatalf("credentials sent on create = %#v, want values from referenced Secret", received)
	}
	var stored harborv1alpha1.PreheatInstance
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get instance after create: %v", err)
	}
	if stored.Status.HarborInstanceID != 55 || stored.Status.HarborStatus != "healthy" || stored.Status.AuthInfoHash == "" {
		t.Fatalf("instance status after create = %#v, want identity, health, and credential hash", stored.Status)
	}
	assertReadyCondition(t, stored.Status.Conditions)
	assertObjectDoesNotContain(t, &stored, "initial-secret")
	assertObjectDoesNotContain(t, &stored, "cache-user")

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("unchanged reconcile: %v", err)
	}
	if len(received) != 2 {
		t.Fatalf("unchanged credentials caused another Harbor write; received %d payloads", len(received))
	}

	storedAuthSecret := &corev1.Secret{}
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: authSecret.Name}, storedAuthSecret); err != nil {
		t.Fatalf("get auth Secret: %v", err)
	}
	storedAuthSecret.Data["password"] = []byte("rotated-secret")
	if err := kubeClient.Update(ctx, storedAuthSecret); err != nil {
		t.Fatalf("rotate auth Secret: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile rotated credentials: %v", err)
	}
	if len(received) != 3 || received[2]["password"] != "rotated-secret" {
		t.Fatalf("rotated credentials sent to Harbor = %#v", received)
	}
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get instance after credential rotation: %v", err)
	}
	rotatedHash := stored.Status.AuthInfoHash
	if rotatedHash == "" || rotatedHash == hashParts(secretDataCanonical(map[string]string{"username": "cache-user", "password": "initial-secret"})) {
		t.Fatalf("credential hash after rotation = %q, want a changed non-empty hash", rotatedHash)
	}

	// A failed Harbor update may echo provider credentials in its response body.
	// The Ready condition must retain a useful error while keeping the secret private.
	storedAuthSecret.Data["password"] = []byte("rotated-secret-2")
	if err := kubeClient.Update(ctx, storedAuthSecret); err != nil {
		t.Fatalf("rotate auth Secret for error case: %v", err)
	}
	failUpdate = true
	if _, err := reconciler.Reconcile(ctx, req); err == nil {
		t.Fatal("failed Harbor update returned no reconcile error")
	} else if strings.Contains(err.Error(), "rotated-secret-2") || strings.Contains(err.Error(), "cache-user") {
		t.Fatalf("reconcile error leaked credentials: %q", err)
	}
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get instance after Harbor update failure: %v", err)
	}
	condition := meta.FindStatusCondition(stored.Status.Conditions, ConditionReady)
	if condition == nil || condition.Status != metav1.ConditionFalse {
		t.Fatalf("Ready condition after failed Harbor update = %#v, want False", condition)
	}
	if strings.Contains(condition.Message, "rotated-secret-2") || strings.Contains(condition.Message, "cache-user") {
		t.Fatalf("Ready condition leaked credentials: %q", condition.Message)
	}
}

//nolint:gocyclo // This test verifies dependency resolution and every policy lifecycle transition.
func TestPreheatPolicyReconcileCreateUpdateNoopAndDelete(t *testing.T) {
	ctx := context.Background()
	var remote *harborclient.PreheatPolicy
	var createCount, updateCount, deleteCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const listPath = "/api/v2.0/projects/library/preheat/policies"
		const itemPath = listPath + "/warm-cache"
		switch {
		case r.Method == http.MethodGet && r.URL.Path == listPath:
			policies := []harborclient.PreheatPolicy{}
			if remote != nil {
				policies = append(policies, *remote)
			}
			writeJSON(t, w, policies)
		case r.Method == http.MethodPost && r.URL.Path == listPath:
			var got harborclient.PreheatPolicy
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode create preheat policy: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			createCount++
			got.ID = 88
			remote = &got
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == itemPath:
			if remote == nil {
				http.NotFound(w, r)
				return
			}
			writeJSON(t, w, remote)
		case r.Method == http.MethodPut && r.URL.Path == itemPath:
			var got harborclient.PreheatPolicy
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode preheat policy update: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			updateCount++
			remote = &got
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && r.URL.Path == itemPath:
			deleteCount++
			remote = nil
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	connection, secret := clusterHarborTestObjects(server.URL)
	binding := &harborv1alpha1.HarborConnectionBinding{Kind: harborv1alpha1.HarborConnectionReferenceKindCluster, Name: "test-harbor"}
	project := &harborv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "library", Namespace: testNamespace},
		Spec:       harborv1alpha1.ProjectSpec{HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef()}},
		Status:     harborv1alpha1.ProjectStatus{HarborProjectID: 123, HarborStatusBase: harborv1alpha1.HarborStatusBase{ResolvedHarborConnection: binding}},
	}
	provider := &harborv1alpha1.PreheatInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "dragonfly"},
		Spec:       harborv1alpha1.PreheatInstanceSpec{HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef()}},
		Status:     harborv1alpha1.PreheatInstanceStatus{HarborInstanceID: 456, HarborStatusBase: harborv1alpha1.HarborStatusBase{ResolvedHarborConnection: binding}},
	}
	policy := &harborv1alpha1.PreheatPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "warm-cache", Namespace: testNamespace},
		Spec: harborv1alpha1.PreheatPolicySpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef()},
			ProjectRef:     harborv1alpha1.ProjectReference{Name: "library"},
			ProviderRef:    harborv1alpha1.PreheatInstanceReference{Name: "dragonfly"},
			Description:    "warm the cache",
			Filters:        `[{"type":"resource","value":"artifact"}]`,
			Trigger:        `{"type":"manual","trigger_settings":{}}`,
			ExtraAttrs:     `{}`,
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, project, provider, policy)
	reconciler := &PreheatPolicyReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: policy.Name}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}
	var stored harborv1alpha1.PreheatPolicy
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get policy after create: %v", err)
	}
	if stored.Status.HarborProjectID != 123 || stored.Status.HarborPreheatPolicyID != 88 || createCount != 1 {
		t.Fatalf("policy IDs/create calls = %d/%d/%d, want 123/88/1", stored.Status.HarborProjectID, stored.Status.HarborPreheatPolicyID, createCount)
	}
	assertReadyCondition(t, stored.Status.Conditions)
	if remote == nil || remote.ProviderID != 456 || remote.ProviderName != provider.Name || remote.ProjectID != 123 {
		t.Fatalf("created Harbor policy dependencies = %#v", remote)
	}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op reconcile: %v", err)
	}
	if updateCount != 0 {
		t.Fatalf("no-op reconcile sent %d updates, want 0", updateCount)
	}

	stored.Spec.Description = "updated description"
	if err := kubeClient.Update(ctx, &stored); err != nil {
		t.Fatalf("update policy spec: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile policy update: %v", err)
	}
	if updateCount != 1 || remote == nil || remote.Description != stored.Spec.Description {
		t.Fatalf("Harbor policy after update = %#v with %d updates", remote, updateCount)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile updated policy again: %v", err)
	}
	if updateCount != 1 {
		t.Fatalf("stable policy sent %d updates, want 1 total", updateCount)
	}

	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete policy: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile policy deletion: %v", err)
	}
	if deleteCount != 1 || remote != nil {
		t.Fatalf("Harbor policy delete calls/state = %d/%#v, want 1/nil", deleteCount, remote)
	}
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); !apierrors.IsNotFound(err) {
		t.Fatalf("policy after finalization error = %v, want NotFound", err)
	}
}

func newAdvancedUnitClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := harborv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add Harbor scheme: %v", err)
	}
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(
			&harborv1alpha1.ClusterHarborConnection{},
			&harborv1alpha1.HarborConnection{},
			&harborv1alpha1.HarborCVEAllowlist{},
			&harborv1alpha1.HarborRole{},
			&harborv1alpha1.Configuration{},
			&harborv1alpha1.HarborUserGroup{},
			&harborv1alpha1.GCSchedule{},
			&harborv1alpha1.ImmutableTagRule{},
			&harborv1alpha1.Label{},
			&harborv1alpha1.Member{},
			&harborv1alpha1.PreheatPolicy{},
			&harborv1alpha1.PreheatInstance{},
			&harborv1alpha1.PurgeAuditSchedule{},
			&harborv1alpha1.Project{},
			&harborv1alpha1.Quota{},
			&harborv1alpha1.Registry{},
			&harborv1alpha1.ReplicationPolicy{},
			&harborv1alpha1.RetentionPolicy{},
			&harborv1alpha1.Robot{},
			&harborv1alpha1.ScanAllSchedule{},
			&harborv1alpha1.ScannerRegistration{},
			&harborv1alpha1.User{},
			&harborv1alpha1.UserGroupClaim{},
			&harborv1alpha1.WebhookPolicy{},
		).
		WithObjects(objects...).
		Build()
}

func clusterHarborTestObjects(baseURL string) (*harborv1alpha1.ClusterHarborConnection, *corev1.Secret) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "harbor-password", Namespace: testNamespace},
		Data:       map[string][]byte{"password": []byte("test-password")},
	}
	connection := &harborv1alpha1.ClusterHarborConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "test-harbor"},
		Spec: harborv1alpha1.HarborConnectionSpec{
			BaseURL: baseURL,
			Credentials: &harborv1alpha1.Credentials{
				Type:     "basic",
				Username: "admin",
				PasswordSecretRef: harborv1alpha1.SecretReference{
					Name: "harbor-password", Namespace: testNamespace, Key: "password",
				},
			},
		},
	}
	return connection, secret
}

func clusterHarborRef() *harborv1alpha1.HarborConnectionReference {
	return &harborv1alpha1.HarborConnectionReference{
		Name: "test-harbor", Kind: harborv1alpha1.HarborConnectionReferenceKindCluster,
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func assertReadyCondition(t *testing.T, conditions []metav1.Condition) {
	t.Helper()
	condition := meta.FindStatusCondition(conditions, ConditionReady)
	if condition == nil || condition.Status != metav1.ConditionTrue {
		t.Fatalf("Ready condition = %#v, want True", condition)
	}
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func assertObjectDoesNotContain(t *testing.T, object any, secret string) {
	t.Helper()
	serialized, err := json.Marshal(object)
	if err != nil {
		t.Fatalf("marshal object for secret check: %v", err)
	}
	if strings.Contains(string(serialized), secret) {
		t.Fatalf("serialized Kubernetes object contains credential %q: %s", secret, serialized)
	}
}

func ptrInt64(value int64) *int64 {
	return &value
}
