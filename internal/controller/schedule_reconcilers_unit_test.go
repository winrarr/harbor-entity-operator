package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

func TestGCScheduleReconcileCreateUpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	var created, updated []harborclient.Schedule
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2.0/system/gc/schedule" {
			http.NotFound(w, r)
			return
		}
		var got harborclient.Schedule
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode GC schedule: %v", err)
			}
		}
		switch r.Method {
		case http.MethodPost:
			created = append(created, got)
			w.WriteHeader(http.StatusCreated)
		case http.MethodPut:
			updated = append(updated, got)
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected GC schedule request %s %s", r.Method, r.URL)
			http.Error(w, "unexpected request", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	connection, secret := clusterHarborTestObjects(server.URL)
	schedule := &harborv1alpha1.GCSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "gc", Namespace: testNamespace},
		Spec: harborv1alpha1.GCScheduleSpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef()},
			Schedule:       harborv1alpha1.ScheduleSpec{Type: harborv1alpha1.ScheduleTypeCustom, Cron: "0 1 * * *"},
			Parameters:     map[string]apiextensionsv1.JSON{"delete_untagged": {Raw: []byte(`true`)}},
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, schedule)
	reconciler := &GCScheduleReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: schedule.Namespace, Name: schedule.Name}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("create GC schedule: %v", err)
	}
	var stored harborv1alpha1.GCSchedule
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get GC schedule after create: %v", err)
	}
	if stored.Status.LastAppliedScheduleHash == "" {
		t.Fatal("GC schedule hash was not persisted")
	}
	assertReadyCondition(t, stored.Status.Conditions)
	if len(created) != 1 || created[0].Parameters["delete_untagged"] != true {
		t.Fatalf("created GC schedule payloads = %#v", created)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op GC schedule reconcile: %v", err)
	}
	if len(created) != 1 || len(updated) != 0 {
		t.Fatalf("no-op GC request counts: creates=%d updates=%d", len(created), len(updated))
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload GC schedule: %v", err)
	}
	stored.Spec.Schedule.Cron = "0 2 * * *"
	if err := kubeClient.Update(ctx, &stored); err != nil {
		t.Fatalf("update GC schedule spec: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("update GC schedule: %v", err)
	}
	if len(updated) != 1 || updated[0].Schedule.Cron != "0 2 * * *" {
		t.Fatalf("updated GC schedule payloads = %#v", updated)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload GC schedule before deletion: %v", err)
	}
	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete GC schedule: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("finalize GC schedule deletion: %v", err)
	}
	if len(created) != 1 || len(updated) != 1 {
		t.Fatalf("deletion unexpectedly changed Harbor schedule: creates=%d updates=%d", len(created), len(updated))
	}
}

func TestPurgeAuditScheduleReconcileMapsParametersAndUpdates(t *testing.T) {
	ctx := context.Background()
	var created, updated []harborclient.Schedule
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2.0/system/purgeaudit/schedule" {
			http.NotFound(w, r)
			return
		}
		var got harborclient.Schedule
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode purge audit schedule: %v", err)
		}
		switch r.Method {
		case http.MethodPost:
			created = append(created, got)
			w.WriteHeader(http.StatusCreated)
		case http.MethodPut:
			updated = append(updated, got)
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected purge audit schedule request %s %s", r.Method, r.URL)
			http.Error(w, "unexpected request", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	connection, secret := clusterHarborTestObjects(server.URL)
	schedule := &harborv1alpha1.PurgeAuditSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "purge", Namespace: testNamespace},
		Spec: harborv1alpha1.PurgeAuditScheduleSpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef()},
			Schedule:       harborv1alpha1.ScheduleSpec{Type: harborv1alpha1.ScheduleTypeDaily, Cron: "0 3 * * *"},
			Parameters:     harborv1alpha1.PurgeAuditParameters{AuditRetentionHour: 168, IncludeEventTypes: "push,pull", DryRun: true},
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, schedule)
	reconciler := &PurgeAuditScheduleReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: schedule.Namespace, Name: schedule.Name}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("create purge audit schedule: %v", err)
	}
	if len(created) != 1 || created[0].Parameters["audit_retention_hour"] != float64(168) || created[0].Parameters["include_event_types"] != "push,pull" || created[0].Parameters["dry_run"] != true {
		t.Fatalf("created purge schedule payloads = %#v", created)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op purge audit schedule reconcile: %v", err)
	}
	if len(created) != 1 || len(updated) != 0 {
		t.Fatalf("no-op purge request counts: creates=%d updates=%d", len(created), len(updated))
	}

	var stored harborv1alpha1.PurgeAuditSchedule
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload purge schedule: %v", err)
	}
	stored.Spec.Parameters.DryRun = false
	if err := kubeClient.Update(ctx, &stored); err != nil {
		t.Fatalf("update purge schedule spec: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("update purge audit schedule: %v", err)
	}
	if len(updated) != 1 || updated[0].Parameters["dry_run"] != nil {
		t.Fatalf("updated purge schedule payloads = %#v", updated)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload purge schedule before deletion: %v", err)
	}
	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete purge schedule: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("finalize purge schedule deletion: %v", err)
	}
	if len(created) != 1 || len(updated) != 1 {
		t.Fatalf("deletion unexpectedly changed Harbor schedule: creates=%d updates=%d", len(created), len(updated))
	}
}

func TestScanAllScheduleReconcileCreatesUpdatesAndAvoidsNoopWrites(t *testing.T) {
	ctx := context.Background()
	var remote *harborclient.Schedule
	var created, updated int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2.0/system/scanAll/schedule" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			if remote == nil {
				http.NotFound(w, r)
				return
			}
			writeJSON(t, w, remote)
		case http.MethodPost, http.MethodPut:
			var got harborclient.Schedule
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode scan all schedule: %v", err)
			}
			remote = &got
			if r.Method == http.MethodPost {
				created++
				w.WriteHeader(http.StatusCreated)
				return
			}
			updated++
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected scan all schedule request %s %s", r.Method, r.URL)
			http.Error(w, "unexpected request", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	connection, secret := clusterHarborTestObjects(server.URL)
	schedule := &harborv1alpha1.ScanAllSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "scan-all", Namespace: testNamespace},
		Spec: harborv1alpha1.ScanAllScheduleSpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef()},
			Schedule:       harborv1alpha1.ScheduleSpec{Type: harborv1alpha1.ScheduleTypeCustom, Cron: "0 4 * * *"},
			Parameters:     map[string]apiextensionsv1.JSON{"漏洞": {Raw: []byte(`{"severity":"High"}`)}},
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, schedule)
	reconciler := &ScanAllScheduleReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: schedule.Namespace, Name: schedule.Name}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("create scan all schedule: %v", err)
	}
	if created != 1 || remote == nil || remote.Schedule.Cron != "0 4 * * *" || remote.Parameters["漏洞"].(map[string]any)["severity"] != "High" {
		t.Fatalf("created scan all schedule count/payload = %d/%#v", created, remote)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op scan all schedule reconcile: %v", err)
	}
	if created != 1 || updated != 0 {
		t.Fatalf("no-op scan all request counts: creates=%d updates=%d", created, updated)
	}

	var stored harborv1alpha1.ScanAllSchedule
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload scan all schedule: %v", err)
	}
	stored.Spec.Schedule.Cron = "0 5 * * *"
	if err := kubeClient.Update(ctx, &stored); err != nil {
		t.Fatalf("update scan all schedule spec: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("update scan all schedule: %v", err)
	}
	if updated != 1 || remote.Schedule.Cron != "0 5 * * *" {
		t.Fatalf("updated scan all schedule count/payload = %d/%#v", updated, remote)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload scan all schedule before deletion: %v", err)
	}
	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete scan all schedule: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("finalize scan all schedule deletion: %v", err)
	}
	if created != 1 || updated != 1 {
		t.Fatalf("deletion unexpectedly changed Harbor schedule: creates=%d updates=%d", created, updated)
	}
}
