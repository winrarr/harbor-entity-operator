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
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

func TestProjectReconcileAdoptsExistingProjectAndDoesNotCreateWhenMissing(t *testing.T) {
	ctx := context.Background()
	for _, existing := range []bool{true, false} {
		t.Run(map[bool]string{true: "adopts matching project", false: "adopt policy rejects missing project"}[existing], func(t *testing.T) {
			var createCalls int
			remote := harborclient.Project{ProjectID: 42, Name: "demo", Metadata: harborclient.ProjectMetadata{Public: "true"}}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/projects":
					if r.URL.Query().Get("q") != "name=demo" {
						t.Errorf("project adoption query = %q", r.URL.RawQuery)
					}
					if existing {
						writeJSON(t, w, []harborclient.Project{remote})
						return
					}
					writeJSON(t, w, []harborclient.Project{})
				case r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/projects/42":
					writeJSON(t, w, remote)
				case r.Method == http.MethodPost && r.URL.Path == "/api/v2.0/projects":
					createCalls++
					t.Errorf("adopt-only Project attempted to create a Harbor project")
					http.Error(w, "unexpected create", http.StatusInternalServerError)
				default:
					t.Errorf("unexpected project adoption request %s %s", r.Method, r.URL)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			connection, secret := clusterHarborTestObjects(server.URL)
			project := &harborv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: testNamespace},
				Spec: harborv1alpha1.ProjectSpec{
					HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef()},
					CreationPolicy: harborv1alpha1.CreationPolicyAdopt,
					Public:         true,
				},
			}
			kubeClient := newAdvancedUnitClient(t, connection, secret, project)
			reconciler := &ProjectReconciler{Client: kubeClient}
			req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: project.Namespace, Name: project.Name}}
			_, err := reconciler.Reconcile(ctx, req)
			if existing {
				if err != nil {
					t.Fatalf("adopt matching project: %v", err)
				}
				var stored harborv1alpha1.Project
				if getErr := kubeClient.Get(ctx, req.NamespacedName, &stored); getErr != nil {
					t.Fatalf("get adopted project: %v", getErr)
				}
				if stored.Status.HarborProjectID != 42 {
					t.Fatalf("adopted Harbor project ID = %d", stored.Status.HarborProjectID)
				}
				assertReadyCondition(t, stored.Status.Conditions)
			} else if err == nil || !strings.Contains(err.Error(), `creationPolicy "Adopt" requires an existing Harbor resource to adopt`) {
				t.Fatalf("missing project under Adopt policy error = %v", err)
			} else {
				var stored harborv1alpha1.Project
				if getErr := kubeClient.Get(ctx, req.NamespacedName, &stored); getErr != nil {
					t.Fatalf("get missing-project status: %v", getErr)
				}
				condition := meta.FindStatusCondition(stored.Status.Conditions, ConditionReady)
				if condition == nil || condition.Status != metav1.ConditionFalse {
					t.Fatalf("missing project Ready condition = %#v", condition)
				}
			}
			if createCalls != 0 {
				t.Fatalf("adopt policy made %d create calls", createCalls)
			}
		})
	}
}
