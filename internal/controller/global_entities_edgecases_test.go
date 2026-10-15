package controller

import (
	"context"
	"errors"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

func TestGlobalHarborEntityAcquireCreationAndAdoptionPolicyMatrix(t *testing.T) {
	tests := []struct {
		name           string
		policy         harborv1alpha1.CreationPolicy
		foundInitially bool
		builtin        bool
		createID       int
		visibleAfter   bool
		findErr        error
		createErr      error
		wantID         int
		wantErr        string
	}{
		{name: "creates when absent and allowed", policy: harborv1alpha1.CreationPolicyCreate, createID: 7, wantID: 7},
		{name: "requires existing resource for adopt policy", policy: harborv1alpha1.CreationPolicyAdopt, wantErr: `creationPolicy "Adopt" requires an existing Harbor resource to adopt`},
		{name: "rejects existing resource under create policy", policy: harborv1alpha1.CreationPolicyCreate, foundInitially: true, wantErr: `already exists; set creationPolicy to Adopt or CreateOrAdopt`},
		{name: "adopts matching custom resource", policy: harborv1alpha1.CreationPolicyAdopt, foundInitially: true, wantID: 11},
		{name: "never adopts a built-in resource", policy: harborv1alpha1.CreationPolicyCreateOrAdopt, foundInitially: true, builtin: true, wantErr: "built-in Harbor testentity"},
		{name: "discovers resource when create returns no ID", policy: harborv1alpha1.CreationPolicyCreateOrAdopt, visibleAfter: true, wantID: 19},
		{name: "reports created resource missing from list", policy: harborv1alpha1.CreationPolicyCreateOrAdopt, wantErr: "not visible in the subsequent list"},
		{name: "propagates find errors", policy: harborv1alpha1.CreationPolicyAdopt, findErr: errors.New("list failed"), wantErr: "list failed"},
		{name: "propagates create errors", policy: harborv1alpha1.CreationPolicyCreate, createErr: errors.New("create failed"), wantErr: "create failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			findCalls := 0
			createCalls := 0
			entity := &harborv1alpha1.HarborRole{ObjectMeta: metav1.ObjectMeta{Name: "reader"}, Spec: harborv1alpha1.HarborRoleSpec{CreationPolicy: test.policy}}
			reconciler := &GlobalHarborEntityReconciler{
				Options: OperatorOptions{},
				Policy: func(obj client.Object) harborv1alpha1.CreationPolicy {
					return obj.(*harborv1alpha1.HarborRole).Spec.CreationPolicy
				},
				ops: globalEntityOperations{
					name: "testentity",
					find: func(context.Context, *harborclient.Client, string) (int, bool, bool, error) {
						findCalls++
						if test.findErr != nil {
							return 0, false, false, test.findErr
						}
						if test.foundInitially && findCalls == 1 {
							return 11, true, test.builtin, nil
						}
						if test.visibleAfter && findCalls > 1 {
							return 19, true, false, nil
						}
						return 0, false, false, nil
					},
					create: func(context.Context, *harborclient.Client, any) (int, error) {
						createCalls++
						return test.createID, test.createErr
					},
				},
			}
			id, err := reconciler.acquire(context.Background(), nil, entity, "desired")
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("acquire error = %v, want %q", err, test.wantErr)
				}
			} else if err != nil || id != test.wantID {
				t.Fatalf("acquire result = %d, %v; want ID %d", id, err, test.wantID)
			}
			if test.foundInitially && test.builtin || (test.foundInitially && test.policy == harborv1alpha1.CreationPolicyAdopt) {
				if createCalls != 0 {
					t.Fatalf("existing entity unexpectedly created %d times", createCalls)
				}
			}
		})
	}
}

func TestGlobalHarborEntityAcquireHonorsOperatorDefaultCreationPolicy(t *testing.T) {
	entity := &harborv1alpha1.HarborRole{ObjectMeta: metav1.ObjectMeta{Name: "reader"}}
	reconciler := &GlobalHarborEntityReconciler{
		Options: OperatorOptions{defaultCreationPolicy: harborv1alpha1.CreationPolicyAdopt},
		Policy: func(obj client.Object) harborv1alpha1.CreationPolicy {
			return obj.(*harborv1alpha1.HarborRole).Spec.CreationPolicy
		},
		ops: globalEntityOperations{
			name: "testentity",
			find: func(context.Context, *harborclient.Client, string) (int, bool, bool, error) {
				return 23, true, false, nil
			},
		},
	}
	id, err := reconciler.acquire(context.Background(), nil, entity, nil)
	if err != nil || id != 23 {
		t.Fatalf("default creation policy adoption = %d, %v", id, err)
	}
	if reconciler.Options.effectiveCreationPolicy("") != harborv1alpha1.CreationPolicyAdopt {
		t.Fatal("empty resource policy did not inherit operator default")
	}
}
