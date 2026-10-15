package controller

import (
	"context"
	"testing"
	"time"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestOperatorOptionsCreationPolicy(t *testing.T) {
	defaults := OperatorOptions{}
	if got := defaults.effectiveCreationPolicy(""); got != harborv1alpha1.CreationPolicyCreate {
		t.Fatalf("effectiveCreationPolicy(\"\") = %q, want %q", got, harborv1alpha1.CreationPolicyCreate)
	}

	options, err := NewOperatorOptions(OperatorConfig{
		DefaultCreationPolicy: harborv1alpha1.CreationPolicyAdopt,
		HarborRequestTimeout:  30 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewOperatorOptions() error = %v", err)
	}
	if got := options.effectiveCreationPolicy(""); got != harborv1alpha1.CreationPolicyAdopt {
		t.Fatalf("effectiveCreationPolicy(\"\") = %q, want %q", got, harborv1alpha1.CreationPolicyAdopt)
	}
	if got := options.effectiveCreationPolicy(harborv1alpha1.CreationPolicyCreateOrAdopt); got != harborv1alpha1.CreationPolicyCreateOrAdopt {
		t.Fatalf("explicit policy = %q, want %q", got, harborv1alpha1.CreationPolicyCreateOrAdopt)
	}
	if _, err := NewOperatorOptions(OperatorConfig{
		DefaultCreationPolicy: "Replace",
		HarborRequestTimeout:  30 * time.Second,
	}); err == nil {
		t.Fatal("NewOperatorOptions() accepted an unsupported policy")
	}
}

func TestOperatorOptionsDriftDetectionInterval(t *testing.T) {
	options, err := NewOperatorOptions(OperatorConfig{
		DefaultCreationPolicy:         harborv1alpha1.CreationPolicyCreate,
		DefaultDriftDetectionInterval: 10 * time.Minute,
		HarborRequestTimeout:          30 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewOperatorOptions() error = %v", err)
	}
	result, err := returnWithDriftDetection(options, &harborv1alpha1.HarborSpecBase{})
	if err != nil {
		t.Fatalf("returnWithDriftDetection() error = %v", err)
	}
	if result.RequeueAfter != 10*time.Minute {
		t.Fatalf("RequeueAfter = %v, want %v", result.RequeueAfter, 10*time.Minute)
	}

	zero := metav1.Duration{}
	result, err = returnWithDriftDetection(options, &harborv1alpha1.HarborSpecBase{DriftDetectionInterval: &zero})
	if err != nil {
		t.Fatalf("returnWithDriftDetection() with explicit zero error = %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("explicit zero RequeueAfter = %v, want 0", result.RequeueAfter)
	}

	oneMinute := metav1.Duration{Duration: time.Minute}
	result, err = returnWithDriftDetection(options, &harborv1alpha1.HarborSpecBase{DriftDetectionInterval: &oneMinute})
	if err != nil {
		t.Fatalf("returnWithDriftDetection() with explicit interval error = %v", err)
	}
	if result.RequeueAfter != time.Minute {
		t.Fatalf("explicit RequeueAfter = %v, want %v", result.RequeueAfter, time.Minute)
	}

	if _, err := NewOperatorOptions(OperatorConfig{
		DefaultCreationPolicy:         harborv1alpha1.CreationPolicyCreate,
		DefaultDriftDetectionInterval: -time.Second,
		HarborRequestTimeout:          30 * time.Second,
	}); err == nil {
		t.Fatal("NewOperatorOptions() accepted a negative drift detection interval")
	}
}

func TestOperatorOptionsHarborRequestTimeout(t *testing.T) {
	if got := (OperatorOptions{}).requestTimeout(); got != 30*time.Second {
		t.Fatalf("requestTimeout() = %v, want %v", got, 30*time.Second)
	}
	options, err := NewOperatorOptions(OperatorConfig{
		DefaultCreationPolicy: harborv1alpha1.CreationPolicyCreate,
		HarborRequestTimeout:  45 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewOperatorOptions() error = %v", err)
	}
	harborClient, err := newHarborClient(options, "https://harbor.example.com", "user", "password", "")
	if err != nil {
		t.Fatalf("newHarborClient() error = %v", err)
	}
	if harborClient.HTTPClient.Timeout != 45*time.Second {
		t.Fatalf("HTTP client timeout = %v, want %v", harborClient.HTTPClient.Timeout, 45*time.Second)
	}
	if _, err := NewOperatorOptions(OperatorConfig{
		DefaultCreationPolicy: harborv1alpha1.CreationPolicyCreate,
	}); err == nil {
		t.Fatal("NewOperatorOptions() accepted a zero request timeout")
	}
}

func TestWithSecretReaderBypassesCachedClientAndReturnsCopiedOptions(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	uncachedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "credentials", Namespace: "team"},
		Data:       map[string][]byte{"token": []byte("fresh-value")},
	}
	uncachedReader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(uncachedSecret).Build()
	cachedClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	original := OperatorOptions{}
	configured := original.WithSecretReader(uncachedReader)

	value, err := readSecretValue(ctx, configured, cachedClient, harborv1alpha1.SecretReference{Name: "credentials", Key: "token"}, "team", "default-key")
	if err != nil || value != "fresh-value" {
		t.Fatalf("read through uncached Secret reader = %q, %v", value, err)
	}
	if original.secretReader != nil {
		t.Fatal("WithSecretReader mutated the original OperatorOptions value")
	}
	if configured.secretReader == nil {
		t.Fatal("WithSecretReader did not configure the returned options")
	}

	var cached corev1.Secret
	if err := client.Reader(cachedClient).Get(ctx, types.NamespacedName{Namespace: "team", Name: "credentials"}, &cached); err == nil {
		t.Fatal("test setup unexpectedly made the uncached Secret visible through the manager client")
	}
}
