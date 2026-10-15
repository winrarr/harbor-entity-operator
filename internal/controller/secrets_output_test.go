package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
)

func TestUpsertOwnedSecretAppliesAndReconcilesOutputMetadata(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	owner := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "team", Name: "robot"}}
	labels := map[string]string{"capsule.clastix.io/tenantresource": "tenant-a"}
	annotations := map[string]string{"example.com/purpose": "registry credentials"}
	if err := upsertOwnedRobotSecretWithMetadata(ctx, c, owner, "shared", "robot-pull", corev1.SecretTypeDockerConfigJson, map[string]string{corev1.DockerConfigJsonKey: "first"}, labels, annotations); err != nil {
		t.Fatalf("create output Secret: %v", err)
	}
	var secret corev1.Secret
	key := types.NamespacedName{Namespace: "shared", Name: "robot-pull"}
	if err := c.Get(ctx, key, &secret); err != nil {
		t.Fatal(err)
	}
	if secret.Labels["capsule.clastix.io/tenantresource"] != "tenant-a" || secret.Annotations["example.com/purpose"] != "registry credentials" {
		t.Fatalf("created Secret metadata = %#v / %#v", secret.Labels, secret.Annotations)
	}
	if secret.Labels[managedByLabelKey] != managedByLabelValue || secret.Annotations[ownerKindAnnotationKey] != "Robot" {
		t.Fatalf("controller ownership metadata was not preserved: %#v / %#v", secret.Labels, secret.Annotations)
	}

	secret.Labels["external.example.com/keep"] = "present"
	if err := c.Update(ctx, &secret); err != nil {
		t.Fatal(err)
	}
	if err := upsertOwnedRobotSecretWithMetadata(ctx, c, owner, "shared", "robot-pull", corev1.SecretTypeDockerConfigJson, map[string]string{corev1.DockerConfigJsonKey: "updated"}, map[string]string{"capsule.clastix.io/tenantresource": "tenant-b"}, nil); err != nil {
		t.Fatalf("reconcile changed output Secret metadata: %v", err)
	}
	if err := c.Get(ctx, key, &secret); err != nil {
		t.Fatal(err)
	}
	if secret.Labels["capsule.clastix.io/tenantresource"] != "tenant-b" || secret.Labels["external.example.com/keep"] != "present" || secret.Annotations["example.com/purpose"] != "" {
		t.Fatalf("reconciled Secret metadata = %#v / %#v", secret.Labels, secret.Annotations)
	}
	if string(secret.Data[corev1.DockerConfigJsonKey]) != "updated" {
		t.Fatalf("Secret data = %q, want updated", secret.Data[corev1.DockerConfigJsonKey])
	}
	if err := upsertOwnedRobotSecretWithMetadata(ctx, c, owner, "shared", "robot-pull", corev1.SecretTypeDockerConfigJson, map[string]string{corev1.DockerConfigJsonKey: "updated"}, nil, nil); err != nil {
		t.Fatalf("remove configured output metadata: %v", err)
	}
	if err := c.Get(ctx, key, &secret); err != nil {
		t.Fatal(err)
	}
	if _, exists := secret.Labels["capsule.clastix.io/tenantresource"]; exists || secret.Labels["external.example.com/keep"] != "present" || secret.Labels[managedByLabelKey] != managedByLabelValue {
		t.Fatalf("removed output metadata damaged unrelated or reserved labels: %#v", secret.Labels)
	}
}

func TestValidateSecretOutputMetadataRejectsReservedAndInvalidMetadata(t *testing.T) {
	tests := []struct {
		name string
		ref  *harborv1alpha1.SecretOutputReference
	}{
		{
			name: "reserved label",
			ref:  &harborv1alpha1.SecretOutputReference{Labels: map[string]string{managedByLabelKey: "spoofed"}},
		},
		{
			name: "reserved ownership annotation",
			ref:  &harborv1alpha1.SecretOutputReference{Annotations: map[string]string{ownerNameAnnotationKey: "spoofed"}},
		},
		{
			name: "reserved tracking annotation",
			ref:  &harborv1alpha1.SecretOutputReference{Annotations: map[string]string{outputMetadataKeysAnn: "[]"}},
		},
		{
			name: "invalid label key",
			ref:  &harborv1alpha1.SecretOutputReference{Labels: map[string]string{"bad key": "value"}},
		},
		{
			name: "invalid label value",
			ref:  &harborv1alpha1.SecretOutputReference{Labels: map[string]string{"example.com/key": "contains spaces"}},
		},
		{
			name: "invalid annotation key",
			ref:  &harborv1alpha1.SecretOutputReference{Annotations: map[string]string{"bad key": "value"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateSecretOutputMetadata(test.ref, "spec.secretRef"); err == nil {
				t.Fatal("invalid metadata was accepted")
			}
		})
	}
}
