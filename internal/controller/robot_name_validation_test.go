package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestBuildRobotCreateRequestChecksPersistedProjectScopedNameLimit(t *testing.T) {
	projectName := strings.Repeat("p", 200)
	scheme := runtime.NewScheme()
	if err := harborv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	project := &harborv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team", Name: projectName},
		Status:     harborv1alpha1.ProjectStatus{HarborProjectID: 42},
	}
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(project).Build()
	reconciler := &RobotReconciler{Client: kubeClient}
	robot := &harborv1alpha1.Robot{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team", Name: strings.Repeat("r", 54)},
		Spec: harborv1alpha1.RobotSpec{
			Level: "project",
			Permissions: []harborv1alpha1.RobotPermission{{
				Kind:       "project",
				ProjectRef: &harborv1alpha1.ProjectReference{Name: projectName},
				Access:     []harborv1alpha1.RobotAccess{{Resource: harborv1alpha1.RobotResourceRepository, Action: harborv1alpha1.RobotActionPull}},
			}},
		},
	}

	if _, err := reconciler.buildRobotCreateRequest(context.Background(), robot); err != nil {
		t.Fatalf("boundary name length 255 rejected: %v", err)
	}
	robot.Name += "r"
	if _, err := reconciler.buildRobotCreateRequest(context.Background(), robot); err == nil {
		t.Fatal("project-scoped effective name length 256 was accepted")
	} else if !strings.Contains(err.Error(), projectName) || !strings.Contains(err.Error(), robot.Name) || !strings.Contains(err.Error(), "255") {
		t.Fatalf("name-limit error omitted actionable details: %v", err)
	}
}

func TestBuildRobotCreateRequestDoesNotApplyProjectNameLimitToSystemRobots(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := harborv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	reconciler := &RobotReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}
	robot := &harborv1alpha1.Robot{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team", Name: strings.Repeat("r", 250)},
		Spec: harborv1alpha1.RobotSpec{
			Level: "system",
			Permissions: []harborv1alpha1.RobotPermission{{
				Kind:   "system",
				Access: []harborv1alpha1.RobotAccess{{Resource: harborv1alpha1.RobotResourceRepository, Action: harborv1alpha1.RobotActionPull}},
			}},
		},
	}
	request, err := reconciler.buildRobotCreateRequest(context.Background(), robot)
	if err != nil {
		t.Fatalf("system-scoped Robot was subjected to project-name limit: %v", err)
	}
	if request.Name != robot.Name {
		t.Fatalf("system-scoped Harbor name = %q, want %q", request.Name, robot.Name)
	}
}

func TestRobotReconcileRejectsLongProjectNameBeforeHarborRequest(t *testing.T) {
	ctx := context.Background()
	projectName := strings.Repeat("p", 200)
	var harborRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		harborRequests++
		t.Errorf("unexpected Harbor request for invalid project-scoped Robot name: %s %s", r.Method, r.URL.Path)
		http.Error(w, "unexpected Harbor request", http.StatusInternalServerError)
	}))
	defer server.Close()

	connection, connectionSecret := clusterHarborTestObjects(server.URL)
	project := &harborv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: projectName, Namespace: testNamespace},
		Status:     harborv1alpha1.ProjectStatus{HarborProjectID: 42},
	}
	robot := &harborv1alpha1.Robot{
		ObjectMeta: metav1.ObjectMeta{Name: strings.Repeat("r", 55), Namespace: testNamespace},
		Spec: harborv1alpha1.RobotSpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef()},
			CreationPolicy: harborv1alpha1.CreationPolicyCreate,
			Level:          "project",
			Duration:       -1,
			SecretRef:      &harborv1alpha1.SecretOutputReference{Name: "robot-token"},
			Permissions: []harborv1alpha1.RobotPermission{{
				Kind:       "project",
				ProjectRef: &harborv1alpha1.ProjectReference{Name: projectName},
				Access:     []harborv1alpha1.RobotAccess{{Resource: harborv1alpha1.RobotResourceRepository, Action: harborv1alpha1.RobotActionPull}},
			}},
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, connectionSecret, project, robot)
	reconciler := &RobotReconciler{Client: kubeClient}
	request := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: robot.Namespace, Name: robot.Name}}

	if _, err := reconciler.Reconcile(ctx, request); err == nil || !strings.Contains(err.Error(), "255") {
		t.Fatalf("reconcile error = %v, want persisted-name limit error", err)
	}
	if harborRequests != 0 {
		t.Fatalf("Harbor requests = %d, want none for invalid project-scoped name", harborRequests)
	}

	var stored harborv1alpha1.Robot
	if err := kubeClient.Get(ctx, request.NamespacedName, &stored); err != nil {
		t.Fatalf("get Robot after rejected reconcile: %v", err)
	}
	condition := findCondition(stored.Status.Conditions, ConditionReady)
	if condition == nil || condition.Status != metav1.ConditionFalse || !strings.Contains(condition.Message, projectName) || !strings.Contains(condition.Message, robot.Name) || !strings.Contains(condition.Message, "255") {
		t.Fatalf("Ready condition = %#v, want actionable project/name/limit error", condition)
	}
}
