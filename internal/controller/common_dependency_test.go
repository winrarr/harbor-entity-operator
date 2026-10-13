package controller

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
)

func TestResolveRegistryIDReturnsDependencyNotReady(t *testing.T) {
	c := newDependencyTestClient(t)
	registry := &harborv1alpha1.Registry{
		ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: "default"},
	}
	if err := c.Create(context.Background(), registry); err != nil {
		t.Fatalf("create registry: %v", err)
	}

	_, err := resolveRegistryID(context.Background(), OperatorOptions{}, c, "default", &harborv1alpha1.RegistryReference{Name: "registry"})
	assertDependencyNotReady(t, err, "Registry", "default/registry")
}

func TestResolveProjectNameReturnsDependencyNotReadyWhenProjectIsMissing(t *testing.T) {
	c := newDependencyTestClient(t)

	_, err := resolveProjectName(context.Background(), OperatorOptions{}, c, "default", &harborv1alpha1.ProjectReference{Name: "project"})
	assertDependencyNotReady(t, err, "Project", "default/project")
}

func TestWaitForDependencyUpdatesStatusAndRequeues(t *testing.T) {
	ctx := context.Background()
	c := newDependencyTestClient(t)
	project := &harborv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "project", Namespace: "default"},
	}
	if err := c.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	err := &dependencyNotReadyError{
		kind:   "Registry",
		key:    clientKey("default", "registry"),
		reason: "status.harborRegistryID is not set yet",
	}
	result, waitErr := waitForDependency(ctx, c, project, &project.Status.HarborStatusBase, 1, err)
	if waitErr != nil {
		t.Fatalf("wait for dependency: %v", waitErr)
	}
	if result.RequeueAfter != dependencyRequeueInterval {
		t.Fatalf("expected requeue after %s, got %s", dependencyRequeueInterval, result.RequeueAfter)
	}

	var got harborv1alpha1.Project
	if err := c.Get(ctx, clientKey("default", "project"), &got); err != nil {
		t.Fatalf("get project: %v", err)
	}
	condition := findCondition(got.Status.Conditions, ConditionReady)
	if condition == nil {
		t.Fatal("expected Ready condition")
	}
	if condition.Reason != "DependencyNotReady" {
		t.Fatalf("expected DependencyNotReady reason, got %q", condition.Reason)
	}
}

func newDependencyTestClient(t *testing.T) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := harborv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add Harbor API scheme: %v", err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&harborv1alpha1.Project{}).Build()
}

func assertDependencyNotReady(t *testing.T, err error, kind, key string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected dependency error")
	}
	var dependencyErr *dependencyNotReadyError
	if !errors.As(err, &dependencyErr) {
		t.Fatalf("expected dependency error, got %T: %v", err, err)
	}
	if dependencyErr.kind != kind || dependencyErr.key.String() != key {
		t.Fatalf("expected %s %s, got %s %s", kind, key, dependencyErr.kind, dependencyErr.key)
	}
}

func clientKey(namespace, name string) types.NamespacedName {
	return types.NamespacedName{Namespace: namespace, Name: name}
}

func findCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}
