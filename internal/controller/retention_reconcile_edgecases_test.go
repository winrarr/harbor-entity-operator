package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
)

func TestRetentionPolicyReconcileRejectsInvalidDesiredStateBeforeHarborWrite(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		trigger *harborv1alpha1.RetentionTrigger
		rules   []harborv1alpha1.RetentionRule
		want    string
	}{
		{name: "missing trigger", want: "spec.trigger is required"},
		{name: "scheduled trigger without cron", trigger: &harborv1alpha1.RetentionTrigger{Kind: "Schedule"}, want: "settings.cron is required"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Errorf("invalid retention policy made an unexpected Harbor request %s %s", r.Method, r.URL)
				http.Error(w, "unexpected write", http.StatusInternalServerError)
			}))
			defer server.Close()
			connection, secret := clusterHarborTestObjects(server.URL)
			policy := &harborv1alpha1.RetentionPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid", Namespace: testNamespace},
				Spec: harborv1alpha1.RetentionPolicySpec{
					HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef()},
					Trigger:        test.trigger,
					Rules:          test.rules,
				},
			}
			kubeClient := newAdvancedUnitClient(t, connection, secret, policy)
			reconciler := &RetentionPolicyReconciler{Client: kubeClient}
			req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: policy.Namespace, Name: policy.Name}}
			_, err := reconciler.Reconcile(ctx, req)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("invalid retention policy error = %v, want %q", err, test.want)
			}
			var stored harborv1alpha1.RetentionPolicy
			if getErr := kubeClient.Get(ctx, req.NamespacedName, &stored); getErr != nil {
				t.Fatalf("get invalid retention status: %v", getErr)
			}
			condition := meta.FindStatusCondition(stored.Status.Conditions, ConditionReady)
			if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "ReconcileError" {
				t.Fatalf("invalid retention Ready condition = %#v", condition)
			}
		})
	}
}

func TestRetentionPolicyReconcileResetsIdentityWhenHarborPolicyDisappears(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2.0/retentions/14" {
			t.Errorf("unexpected retention request %s %s", r.Method, r.URL)
			http.NotFound(w, r)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	connection, secret := clusterHarborTestObjects(server.URL)
	policy := &harborv1alpha1.RetentionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "lost", Namespace: testNamespace},
		Spec: harborv1alpha1.RetentionPolicySpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef()},
			Trigger:        &harborv1alpha1.RetentionTrigger{Kind: "Manual"},
			Rules:          []harborv1alpha1.RetentionRule{{Action: "retain", Template: "always"}},
		},
		Status: harborv1alpha1.RetentionPolicyStatus{HarborRetentionID: 14},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, policy)
	reconciler := &RetentionPolicyReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: policy.Namespace, Name: policy.Name}}
	result, err := reconciler.Reconcile(ctx, req)
	if err != nil || result.RequeueAfter != dependencyRequeueInterval {
		t.Fatalf("missing retention reconcile result/error = %#v/%v", result, err)
	}
	var stored harborv1alpha1.RetentionPolicy
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get retention after remote deletion: %v", err)
	}
	if stored.Status.HarborRetentionID != 0 {
		t.Fatalf("Harbor retention ID after remote deletion = %d, want 0", stored.Status.HarborRetentionID)
	}
	condition := meta.FindStatusCondition(stored.Status.Conditions, ConditionReady)
	if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "NotFound" {
		t.Fatalf("remote retention deletion Ready condition = %#v", condition)
	}
}
