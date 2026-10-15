package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
)

const (
	robotSecretOwnerKind   = "Robot"
	managedByLabelKey      = "harbor.harbor-entity-operator.io/managed-by"
	managedByLabelValue    = "harbor-entity-operator"
	ownerKindAnnotationKey = "harbor.harbor-entity-operator.io/owner-kind"
	ownerNameAnnotationKey = "harbor.harbor-entity-operator.io/owner-name"
	ownerNamespaceAnnKey   = "harbor.harbor-entity-operator.io/owner-namespace"
	outputMetadataKeysAnn  = "harbor.harbor-entity-operator.io/output-metadata-keys"
)

var reservedSecretOutputLabels = map[string]struct{}{
	managedByLabelKey: {},
}

var reservedSecretOutputAnnotations = map[string]struct{}{
	ownerKindAnnotationKey: {},
	ownerNameAnnotationKey: {},
	ownerNamespaceAnnKey:   {},
	outputMetadataKeysAnn:  {},
}

func readSecretValue(ctx context.Context, options OperatorOptions, c client.Client, ref harborv1alpha1.SecretReference, defaultNamespace, defaultKey string) (string, error) {
	namespace := ref.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	}
	if err := validateReferenceNamespace(options, defaultNamespace, namespace, "Secret"); err != nil {
		return "", err
	}
	key := ref.Key
	if key == "" {
		key = defaultKey
	}
	if key == "" {
		return "", fmt.Errorf("secret key must be set for %s/%s", namespace, ref.Name)
	}

	var secret corev1.Secret
	reader := client.Reader(c)
	if uncached := options.secretReader; uncached != nil {
		reader = uncached
	}
	if err := reader.Get(ctx, types.NamespacedName{Namespace: namespace, Name: ref.Name}, &secret); err != nil {
		return "", err
	}
	value, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("key %s not found in secret %s/%s", key, namespace, ref.Name)
	}
	return string(value), nil
}

func hashSecret(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func upsertOwnedRobotSecretWithMetadata(
	ctx context.Context,
	c client.Client,
	owner client.Object,
	namespace, name string,
	secretType corev1.SecretType,
	values map[string]string,
	labels, annotations map[string]string,
) error {
	for key := range values {
		if key == "" {
			return fmt.Errorf("secret key must be set for %s/%s", namespace, name)
		}
	}
	var secret corev1.Secret
	err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &secret)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		secret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
			Type: secretType,
			Data: map[string][]byte{},
		}
		setSecretValues(&secret, values)
		ensureRobotSecretOwnershipMetadata(&secret, owner)
		if _, err := applySecretOutputMetadata(&secret, labels, annotations); err != nil {
			return err
		}
		return c.Create(ctx, &secret)
	}
	if !robotSecretOwnedBy(&secret, owner) {
		return fmt.Errorf("secret %s/%s already exists and is not managed by Robot %s/%s", namespace, name, owner.GetNamespace(), owner.GetName())
	}

	changed := ensureRobotSecretOwnershipMetadata(&secret, owner)
	if secret.Type != secretType {
		secret.Type = secretType
		changed = true
	}
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
		changed = true
	}
	if setSecretValues(&secret, values) {
		changed = true
	}
	metadataChanged, err := applySecretOutputMetadata(&secret, labels, annotations)
	if err != nil {
		return err
	}
	changed = metadataChanged || changed
	if !changed {
		return nil
	}
	return c.Update(ctx, &secret)
}

func validateSecretOutputMetadata(ref *harborv1alpha1.SecretOutputReference, fieldPath string) error {
	if ref == nil {
		return nil
	}
	for key, value := range ref.Labels {
		if _, reserved := reservedSecretOutputLabels[key]; reserved {
			return fmt.Errorf("%s.labels cannot set reserved operator label %q", fieldPath, key)
		}
		if problems := validation.IsQualifiedName(key); len(problems) > 0 {
			return fmt.Errorf("%s.labels has invalid key %q: %s", fieldPath, key, problems[0])
		}
		if problems := validation.IsValidLabelValue(value); len(problems) > 0 {
			return fmt.Errorf("%s.labels has invalid value for key %q: %s", fieldPath, key, problems[0])
		}
	}
	for key := range ref.Annotations {
		if _, reserved := reservedSecretOutputAnnotations[key]; reserved {
			return fmt.Errorf("%s.annotations cannot set reserved operator annotation %q", fieldPath, key)
		}
		if problems := validation.IsQualifiedName(key); len(problems) > 0 {
			return fmt.Errorf("%s.annotations has invalid key %q: %s", fieldPath, key, problems[0])
		}
	}
	return nil
}

type secretOutputMetadataKeys struct {
	Labels      []string `json:"labels,omitempty"`
	Annotations []string `json:"annotations,omitempty"`
}

func applySecretOutputMetadata(secret *corev1.Secret, labels, annotations map[string]string) (bool, error) {
	var previous secretOutputMetadataKeys
	if raw := secret.Annotations[outputMetadataKeysAnn]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &previous); err != nil {
			return false, fmt.Errorf("parse managed output metadata keys on Secret %s/%s: %w", secret.Namespace, secret.Name, err)
		}
	}
	for key := range labels {
		if _, reserved := reservedSecretOutputLabels[key]; reserved {
			return false, fmt.Errorf("secret output labels cannot set reserved operator label %q", key)
		}
	}
	for key := range annotations {
		if _, reserved := reservedSecretOutputAnnotations[key]; reserved {
			return false, fmt.Errorf("secret output annotations cannot set reserved operator annotation %q", key)
		}
	}

	changed := false
	if len(labels) > 0 || len(annotations) > 0 || len(previous.Labels) > 0 || len(previous.Annotations) > 0 {
		if secret.Labels == nil {
			secret.Labels = map[string]string{}
		}
		if secret.Annotations == nil {
			secret.Annotations = map[string]string{}
		}
	}
	for _, key := range previous.Labels {
		if _, remains := labels[key]; !remains {
			if _, reserved := reservedSecretOutputLabels[key]; !reserved {
				if _, exists := secret.Labels[key]; exists {
					delete(secret.Labels, key)
					changed = true
				}
			}
		}
	}
	for _, key := range previous.Annotations {
		if _, remains := annotations[key]; !remains {
			if _, reserved := reservedSecretOutputAnnotations[key]; !reserved {
				if _, exists := secret.Annotations[key]; exists {
					delete(secret.Annotations, key)
					changed = true
				}
			}
		}
	}
	for key, value := range labels {
		if secret.Labels[key] != value {
			secret.Labels[key] = value
			changed = true
		}
	}
	for key, value := range annotations {
		if secret.Annotations[key] != value {
			secret.Annotations[key] = value
			changed = true
		}
	}

	managed := secretOutputMetadataKeys{Labels: sortedMapKeys(labels), Annotations: sortedMapKeys(annotations)}
	if len(managed.Labels) == 0 && len(managed.Annotations) == 0 {
		if _, exists := secret.Annotations[outputMetadataKeysAnn]; exists {
			delete(secret.Annotations, outputMetadataKeysAnn)
			changed = true
		}
		return changed, nil
	}
	raw, err := json.Marshal(managed)
	if err != nil {
		return false, fmt.Errorf("encode managed output metadata keys: %w", err)
	}
	if secret.Annotations[outputMetadataKeysAnn] != string(raw) {
		secret.Annotations[outputMetadataKeysAnn] = string(raw)
		changed = true
	}
	return changed, nil
}

func sortedMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func setSecretValues(secret *corev1.Secret, values map[string]string) bool {
	changed := false
	for key, value := range values {
		if existing, ok := secret.Data[key]; !ok || string(existing) != value {
			secret.Data[key] = []byte(value)
			changed = true
		}
	}
	return changed
}

func robotSecretOwnedBy(secret *corev1.Secret, owner client.Object) bool {
	return secret.Labels[managedByLabelKey] == managedByLabelValue &&
		secret.Annotations[ownerKindAnnotationKey] == robotSecretOwnerKind &&
		secret.Annotations[ownerNamespaceAnnKey] == owner.GetNamespace() &&
		secret.Annotations[ownerNameAnnotationKey] == owner.GetName()
}

func ensureRobotSecretOwnershipMetadata(secret *corev1.Secret, owner client.Object) bool {
	changed := false
	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}
	if secret.Annotations == nil {
		secret.Annotations = map[string]string{}
	}
	if secret.Labels[managedByLabelKey] != managedByLabelValue {
		secret.Labels[managedByLabelKey] = managedByLabelValue
		changed = true
	}
	if secret.Annotations[ownerKindAnnotationKey] != robotSecretOwnerKind {
		secret.Annotations[ownerKindAnnotationKey] = robotSecretOwnerKind
		changed = true
	}
	if secret.Annotations[ownerNamespaceAnnKey] != owner.GetNamespace() {
		secret.Annotations[ownerNamespaceAnnKey] = owner.GetNamespace()
		changed = true
	}
	if secret.Annotations[ownerNameAnnotationKey] != owner.GetName() {
		secret.Annotations[ownerNameAnnotationKey] = owner.GetName()
		changed = true
	}
	return changed
}
