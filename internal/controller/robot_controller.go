package controller

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

const robotUsernameSecretKey = "username"

type RobotReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Options OperatorOptions
	logger  logr.Logger
}

// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=robots,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=robots/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=robots/finalizers,verbs=update
// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=projects,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=harborconnections;clusterharborconnections,verbs=get;list;watch

func (r *RobotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger = log.FromContext(ctx).WithName(fmt.Sprintf("[Robot:%s]", req.NamespacedName))

	var cr harborv1alpha1.Robot
	if found, err := loadResource(ctx, r.Client, req.NamespacedName, &cr, r.logger); err != nil {
		return ctrl.Result{}, err
	} else if !found {
		return ctrl.Result{}, nil
	}

	if err := markReconcilingIfNeeded(ctx, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Generation); err != nil {
		return ctrl.Result{}, err
	}

	hc, err := getHarborClientForObject(ctx, r.Options, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Namespace, cr.Spec.HarborConnectionRef)
	if err != nil {
		if done, finalErr := finalizeWithoutHarborConnection(ctx, r.Client, &cr, cr.Spec.GetDeletionPolicy(), true, err); done {
			return ctrl.Result{}, finalErr
		}
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Generation, err)
	}

	if done, err := finalizeIfDeleting(ctx, r.Client, &cr, cr.Spec.GetDeletionPolicy(), func() error {
		return r.deleteRobot(ctx, hc, &cr)
	}); done {
		return ctrl.Result{}, err
	}

	if err := ensureFinalizer(ctx, r.Client, &cr); err != nil {
		return ctrl.Result{}, err
	}

	if err := validateRobotSpec(&cr); err != nil {
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Generation, err)
	}

	secretRef, err := resolveRobotSecretRef(&cr)
	if err != nil {
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Generation, err)
	}
	imagePullSecretRef, err := resolveImagePullSecretRef(&cr, secretRef)
	if err != nil {
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Generation, err)
	}

	if cr.Status.HarborRobotID == 0 && allowsAdoption(r.Options, cr.Spec.CreationPolicy) {
		if ok, err := r.adoptExisting(ctx, hc, &cr); err != nil {
			return ctrl.Result{}, setErrorStatus(ctx, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Generation, err)
		} else if ok {
			r.logger.Info("Adopted robot", "ID", cr.Status.HarborRobotID)
		}
	}

	if cr.Status.HarborRobotID == 0 {
		if err := requireCreationAllowed(r.Options, cr.Spec.CreationPolicy); err != nil {
			return ctrl.Result{}, setErrorStatus(ctx, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Generation, err)
		}
		return r.createRobot(ctx, hc, &cr, secretRef, imagePullSecretRef)
	}

	return r.reconcileExisting(ctx, hc, &cr, secretRef, imagePullSecretRef)
}

func (r *RobotReconciler) createRobot(
	ctx context.Context,
	hc *harborclient.Client,
	cr *harborv1alpha1.Robot,
	secretRef *harborv1alpha1.SecretReference,
	imagePullSecretRef *harborv1alpha1.SecretObjectReference,
) (ctrl.Result, error) {
	createReq, err := r.buildRobotCreateRequest(ctx, cr)
	if err != nil {
		if isDependencyNotReady(err) {
			return waitForDependency(ctx, r.Client, cr, &cr.Status.HarborStatusBase, cr.Generation, err)
		}
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, cr, &cr.Status.HarborStatusBase, cr.Generation, err)
	}
	created, err := hc.CreateRobot(ctx, createReq)
	if err != nil {
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, cr, &cr.Status.HarborStatusBase, cr.Generation, err)
	}
	cr.Status.HarborRobotID = created.ID
	cr.Status.Username = created.Name
	storedSecret := created.Secret
	if storedSecret == "" {
		storedSecret, err = rotateRobotSecret(ctx, hc, cr.Status.HarborRobotID)
		if err != nil {
			return ctrl.Result{}, setErrorStatus(ctx, r.Client, cr, &cr.Status.HarborStatusBase, cr.Generation, err)
		}
	}
	if err := upsertRobotSecret(ctx, r.Client, cr, secretRef, created.Name, storedSecret); err != nil {
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, cr, &cr.Status.HarborStatusBase, cr.Generation, err)
	}
	if err := r.reconcileImagePullSecret(ctx, hc, cr, imagePullSecretRef, storedSecret); err != nil {
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, cr, &cr.Status.HarborStatusBase, cr.Generation, err)
	}
	now := metav1.Now()
	cr.Status.LastRotatedAt = &now
	if err := setReadyStatus(ctx, r.Client, cr, &cr.Status.HarborStatusBase, cr.Generation, "Created", "Robot created"); err != nil {
		return ctrl.Result{}, err
	}
	r.logger.Info("Created robot", "ID", created.ID)
	return returnWithDriftDetection(r.Options, &cr.Spec.HarborSpecBase)
}

func (r *RobotReconciler) reconcileExisting(
	ctx context.Context,
	hc *harborclient.Client,
	cr *harborv1alpha1.Robot,
	secretRef *harborv1alpha1.SecretReference,
	imagePullSecretRef *harborv1alpha1.SecretObjectReference,
) (ctrl.Result, error) {
	current, err := hc.GetRobotByID(ctx, cr.Status.HarborRobotID)
	if err != nil {
		if harborclient.IsNotFound(err) {
			return requeueOnRemoteNotFound(ctx, r.Client, cr, &cr.Status.HarborStatusBase, cr.Generation, func() {
				cr.Status.HarborRobotID = 0
			}, "Robot not found in Harbor")
		}
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, cr, &cr.Status.HarborStatusBase, cr.Generation, err)
	}

	if !robotLevelMatches(cr.Spec.Level, current.Level) {
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, cr, &cr.Status.HarborStatusBase, cr.Generation, fmt.Errorf("robot level mismatch: desired %q, current %q", cr.Spec.Level, current.Level))
	}
	if current.Name == "" {
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, cr, &cr.Status.HarborStatusBase, cr.Generation, fmt.Errorf("harbor returned an empty robot username"))
	}
	statusChanged := setRobotUsernameStatus(cr, current.Name)
	if err := syncRobotUsernameSecret(ctx, r.Client, cr, secretRef, current.Name); err != nil {
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, cr, &cr.Status.HarborStatusBase, cr.Generation, err)
	}
	desired, err := r.buildRobotUpdateRequest(ctx, cr, current)
	if err != nil {
		if isDependencyNotReady(err) {
			return waitForDependency(ctx, r.Client, cr, &cr.Status.HarborStatusBase, cr.Generation, err)
		}
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, cr, &cr.Status.HarborStatusBase, cr.Generation, err)
	}
	if robotNeedsUpdate(desired, current) {
		if err := hc.UpdateRobot(ctx, current.ID, desired); err != nil {
			return ctrl.Result{}, setErrorStatus(ctx, r.Client, cr, &cr.Status.HarborStatusBase, cr.Generation, err)
		}
		r.logger.Info("Updated robot", "ID", current.ID)
	}

	statusChanged = updateRobotExpiryStatus(cr, current.ExpiresAt) || statusChanged

	var imagePullToken string
	if shouldRotateRobot(cr) {
		rotatedSecret, err := rotateRobotSecret(ctx, hc, current.ID)
		if err != nil {
			return ctrl.Result{}, setErrorStatus(ctx, r.Client, cr, &cr.Status.HarborStatusBase, cr.Generation, err)
		}
		if err := upsertRobotSecret(ctx, r.Client, cr, secretRef, current.Name, rotatedSecret); err != nil {
			return ctrl.Result{}, setErrorStatus(ctx, r.Client, cr, &cr.Status.HarborStatusBase, cr.Generation, err)
		}
		imagePullToken = rotatedSecret
		now := metav1.Now()
		cr.Status.LastRotatedAt = &now
		refreshed, err := hc.GetRobotByID(ctx, current.ID)
		if err == nil {
			updateRobotExpiryStatus(cr, refreshed.ExpiresAt)
		}
		statusChanged = true
		r.logger.Info("Rotated robot secret", "ID", current.ID)
	} else if imagePullSecretRef != nil {
		if secretRef != nil {
			imagePullToken, err = readSecretValue(ctx, r.Options, r.Client, *secretRef, cr.Namespace, "secret")
		} else {
			imagePullToken, err = r.readImagePullSecretToken(ctx, hc, cr, imagePullSecretRef)
		}
		if err != nil {
			return ctrl.Result{}, setErrorStatus(ctx, r.Client, cr, &cr.Status.HarborStatusBase, cr.Generation, err)
		}
	}

	if imagePullSecretRef != nil {
		if err := r.reconcileImagePullSecret(ctx, hc, cr, imagePullSecretRef, imagePullToken); err != nil {
			return ctrl.Result{}, setErrorStatus(ctx, r.Client, cr, &cr.Status.HarborStatusBase, cr.Generation, err)
		}
	}

	condChanged := markReady(&cr.Status.HarborStatusBase, cr.Generation, "Reconciled", "Robot reconciled")
	if statusChanged || condChanged {
		if err := r.Status().Update(ctx, cr); err != nil {
			return ctrl.Result{}, err
		}
	}

	return returnWithDriftDetection(r.Options, &cr.Spec.HarborSpecBase)
}

func upsertRobotSecret(ctx context.Context, c client.Client, cr *harborv1alpha1.Robot, ref *harborv1alpha1.SecretReference, username, token string) error {
	if ref == nil {
		return nil
	}
	if username == "" {
		return fmt.Errorf("harbor returned an empty robot username")
	}
	return upsertOwnedSecretValues(ctx, c, cr, "Robot", ref.Namespace, ref.Name, map[string]string{
		ref.Key:                token,
		robotUsernameSecretKey: username,
	})
}

type dockerConfig struct {
	Auths map[string]dockerConfigAuth `json:"auths"`
}

type dockerConfigAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Auth     string `json:"auth"`
}

func (r *RobotReconciler) reconcileImagePullSecret(
	ctx context.Context,
	hc *harborclient.Client,
	cr *harborv1alpha1.Robot,
	ref *harborv1alpha1.SecretObjectReference,
	token string,
) error {
	if ref == nil {
		return nil
	}
	if token == "" {
		return fmt.Errorf("robot secret is empty")
	}

	namespace := ref.Namespace
	if namespace == "" {
		namespace = cr.Namespace
	}
	if err := validateReferenceNamespace(r.Options, cr.Namespace, namespace, "image pull Secret"); err != nil {
		return err
	}
	registryHost, err := harborRegistryHost(hc.BaseURL)
	if err != nil {
		return err
	}

	encodedAuth := base64.StdEncoding.EncodeToString([]byte(cr.Status.Username + ":" + token))
	config, err := json.Marshal(dockerConfig{
		Auths: map[string]dockerConfigAuth{
			registryHost: {
				Username: cr.Status.Username,
				Password: token,
				Auth:     encodedAuth,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("marshal image pull Secret: %w", err)
	}

	return upsertOwnedSecret(ctx, r.Client, cr, "Robot", namespace, ref.Name, corev1.SecretTypeDockerConfigJson, map[string]string{
		corev1.DockerConfigJsonKey: string(config),
	})
}

func (r *RobotReconciler) readImagePullSecretToken(
	ctx context.Context,
	hc *harborclient.Client,
	cr *harborv1alpha1.Robot,
	ref *harborv1alpha1.SecretObjectReference,
) (string, error) {
	namespace := ref.Namespace
	if namespace == "" {
		namespace = cr.Namespace
	}
	if err := validateReferenceNamespace(r.Options, cr.Namespace, namespace, "image pull Secret"); err != nil {
		return "", err
	}

	var secret corev1.Secret
	reader := client.Reader(r.Client)
	if uncached := r.Options.secretReader; uncached != nil {
		reader = uncached
	}
	if err := reader.Get(ctx, types.NamespacedName{Namespace: namespace, Name: ref.Name}, &secret); err != nil {
		return "", err
	}
	if !secretOwnedBy(&secret, cr, "Robot") {
		return "", fmt.Errorf("secret %s/%s already exists and is not managed by Robot %s/%s", namespace, ref.Name, cr.Namespace, cr.Name)
	}
	if secret.Type != corev1.SecretTypeDockerConfigJson {
		return "", fmt.Errorf("image pull Secret %s/%s has type %q, expected %q", namespace, ref.Name, secret.Type, corev1.SecretTypeDockerConfigJson)
	}

	rawConfig, ok := secret.Data[corev1.DockerConfigJsonKey]
	if !ok || len(rawConfig) == 0 {
		return "", fmt.Errorf("image pull Secret %s/%s does not contain %s", namespace, ref.Name, corev1.DockerConfigJsonKey)
	}
	var config dockerConfig
	if err := json.Unmarshal(rawConfig, &config); err != nil {
		return "", fmt.Errorf("parse image pull Secret %s/%s: %w", namespace, ref.Name, err)
	}
	registryHost, err := harborRegistryHost(hc.BaseURL)
	if err != nil {
		return "", err
	}
	credentials, ok := config.Auths[registryHost]
	if !ok || credentials.Password == "" {
		return "", fmt.Errorf("image pull Secret %s/%s has no credentials for Harbor registry %q", namespace, ref.Name, registryHost)
	}
	return credentials.Password, nil
}

func harborRegistryHost(baseURL string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse Harbor baseURL: %w", err)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("harbor baseURL %q has no host", baseURL)
	}
	return parsed.Host, nil
}

func syncRobotUsernameSecret(ctx context.Context, c client.Client, cr *harborv1alpha1.Robot, ref *harborv1alpha1.SecretReference, username string) error {
	if ref == nil {
		return nil
	}
	var secret corev1.Secret
	err := c.Get(ctx, types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, &secret)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !secretOwnedBy(&secret, cr, "Robot") {
		return fmt.Errorf("secret %s/%s already exists and is not managed by Robot %s/%s", ref.Namespace, ref.Name, cr.Namespace, cr.Name)
	}
	return upsertOwnedSecretValues(ctx, c, cr, "Robot", ref.Namespace, ref.Name, map[string]string{robotUsernameSecretKey: username})
}

func setRobotUsernameStatus(cr *harborv1alpha1.Robot, username string) bool {
	if cr.Status.Username == username {
		return false
	}
	cr.Status.Username = username
	return true
}
func (r *RobotReconciler) deleteRobot(ctx context.Context, hc *harborclient.Client, cr *harborv1alpha1.Robot) error {
	if cr.Status.HarborRobotID == 0 {
		return nil
	}
	return hc.DeleteRobot(ctx, cr.Status.HarborRobotID)
}

func (r *RobotReconciler) adoptExisting(ctx context.Context, hc *harborclient.Client, cr *harborv1alpha1.Robot) (bool, error) {
	query := "name=" + cr.Name
	robots, err := hc.ListRobots(ctx, query)
	if err != nil {
		return false, err
	}
	if ok, err := r.findAndAdoptRobot(ctx, cr, robots); ok || err != nil {
		return ok, err
	}

	robots, err = hc.ListRobots(ctx, "")
	if err != nil {
		return false, err
	}
	return r.findAndAdoptRobot(ctx, cr, robots)
}

func (r *RobotReconciler) findAndAdoptRobot(ctx context.Context, cr *harborv1alpha1.Robot, robots []harborclient.Robot) (bool, error) {
	for _, robot := range robots {
		if robotNameMatches(cr.Name, robot.Name) && robotLevelMatches(cr.Spec.Level, robot.Level) {
			cr.Status.HarborRobotID = robot.ID
			return true, r.Status().Update(ctx, cr)
		}
	}
	return false, nil
}

func validateRobotSpec(cr *harborv1alpha1.Robot) error {
	if cr.Spec.Level == "" {
		return fmt.Errorf("spec.level is required")
	}
	if len(cr.Spec.Permissions) == 0 {
		return fmt.Errorf("spec.permissions must contain at least one permission")
	}
	if cr.Spec.Duration == 0 {
		return fmt.Errorf("spec.duration must be either -1 or a positive integer")
	}
	if cr.Spec.SecretRef == nil && cr.Spec.ImagePullSecretRef == nil {
		return fmt.Errorf("at least one of spec.secretRef or spec.imagePullSecretRef must be set")
	}
	return nil
}

func resolveRobotSecretRef(cr *harborv1alpha1.Robot) (*harborv1alpha1.SecretReference, error) {
	if cr.Spec.SecretRef == nil {
		return nil, nil
	}
	if cr.Spec.SecretRef.Name == "" {
		return nil, fmt.Errorf("spec.secretRef.name is required when secretRef is set")
	}
	ref := *cr.Spec.SecretRef
	if ref.Namespace == "" {
		ref.Namespace = cr.Namespace
	}
	if ref.Key == "" {
		ref.Key = "secret"
	}
	if ref.Key == robotUsernameSecretKey {
		return nil, fmt.Errorf("spec.secretRef.key %q is reserved for the canonical Harbor username", ref.Key)
	}
	return &ref, nil
}

func resolveImagePullSecretRef(cr *harborv1alpha1.Robot, robotSecretRef *harborv1alpha1.SecretReference) (*harborv1alpha1.SecretObjectReference, error) {
	if cr.Spec.ImagePullSecretRef == nil {
		return nil, nil
	}
	ref := *cr.Spec.ImagePullSecretRef
	if ref.Name == "" {
		return nil, fmt.Errorf("spec.imagePullSecretRef.name is required")
	}
	if ref.Namespace == "" {
		ref.Namespace = cr.Namespace
	}
	if robotSecretRef != nil && ref.Namespace == robotSecretRef.Namespace && ref.Name == robotSecretRef.Name {
		return nil, fmt.Errorf("spec.imagePullSecretRef must reference a different Secret than spec.secretRef")
	}
	return &ref, nil
}

func (r *RobotReconciler) buildRobotCreateRequest(ctx context.Context, cr *harborv1alpha1.Robot) (harborclient.RobotCreateRequest, error) {
	permissions, err := r.buildRobotPermissions(ctx, cr)
	if err != nil {
		return harborclient.RobotCreateRequest{}, err
	}
	return harborclient.RobotCreateRequest{
		Name:        cr.Name,
		Description: cr.Spec.Description,
		Secret:      "",
		Level:       cr.Spec.Level,
		Disable:     cr.Spec.Disable,
		Duration:    &cr.Spec.Duration,
		Permissions: permissions,
	}, nil
}

func (r *RobotReconciler) buildRobotUpdateRequest(ctx context.Context, cr *harborv1alpha1.Robot, current *harborclient.Robot) (harborclient.Robot, error) {
	permissions, err := r.buildRobotPermissions(ctx, cr)
	if err != nil {
		return harborclient.Robot{}, err
	}
	duration := cr.Spec.Duration
	if duration == 0 {
		duration = -1
	}
	desired := harborclient.Robot{
		ID:          current.ID,
		Name:        current.Name,
		Description: cr.Spec.Description,
		Level:       current.Level,
		Disable:     current.Disable,
		Duration:    current.Duration,
		Permissions: permissions,
	}
	if cr.Spec.Disable != nil {
		desired.Disable = *cr.Spec.Disable
	}
	desired.Duration = &duration
	return desired, nil
}

func robotNeedsUpdate(desired harborclient.Robot, current *harborclient.Robot) bool {
	if desired.Description != current.Description {
		return true
	}
	if desired.Disable != current.Disable {
		return true
	}
	if !intPtrEqual(desired.Duration, current.Duration) {
		return true
	}
	return !robotPermissionsEqual(desired.Permissions, current.Permissions)
}

func robotPermissionsEqual(a, b []harborclient.RobotPermission) bool {
	normA := normalizeRobotPermissions(a)
	normB := normalizeRobotPermissions(b)
	if len(normA) != len(normB) {
		return false
	}
	for i := range normA {
		if normA[i].Kind != normB[i].Kind || normA[i].Namespace != normB[i].Namespace {
			return false
		}
		if len(normA[i].Access) != len(normB[i].Access) {
			return false
		}
		for j := range normA[i].Access {
			if normA[i].Access[j] != normB[i].Access[j] {
				return false
			}
		}
	}
	return true
}

func normalizeRobotPermissions(perms []harborclient.RobotPermission) []harborclient.RobotPermission {
	out := make([]harborclient.RobotPermission, 0, len(perms))
	for _, perm := range perms {
		access := make([]harborclient.Access, len(perm.Access))
		copy(access, perm.Access)
		sort.Slice(access, func(i, j int) bool {
			if access[i].Resource != access[j].Resource {
				return access[i].Resource < access[j].Resource
			}
			if access[i].Action != access[j].Action {
				return access[i].Action < access[j].Action
			}
			return access[i].Effect < access[j].Effect
		})
		out = append(out, harborclient.RobotPermission{
			Kind:      perm.Kind,
			Namespace: perm.Namespace,
			Access:    access,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Namespace < out[j].Namespace
	})
	return out
}

func (r *RobotReconciler) buildRobotPermissions(ctx context.Context, cr *harborv1alpha1.Robot) ([]harborclient.RobotPermission, error) {
	perms := make([]harborclient.RobotPermission, 0, len(cr.Spec.Permissions))
	for _, perm := range cr.Spec.Permissions {
		access := make([]harborclient.Access, 0, len(perm.Access))
		for _, rule := range perm.Access {
			effect := rule.Effect
			if effect == "" {
				effect = "allow"
			}
			access = append(access, harborclient.Access{
				Resource: string(rule.Resource),
				Action:   string(rule.Action),
				Effect:   effect,
			})
		}
		namespace := ""
		if strings.EqualFold(perm.Kind, "project") {
			if perm.ProjectRef == nil {
				namespace = "*"
			} else {
				var err error
				namespace, err = resolveProjectName(ctx, r.Options, r.Client, cr.Namespace, perm.ProjectRef)
				if err != nil {
					return nil, err
				}
			}
		} else if strings.EqualFold(perm.Kind, "system") {
			if perm.ProjectRef != nil {
				return nil, fmt.Errorf("projectRef is only valid for project-scoped robot permissions")
			}
			namespace = "/"
		} else if perm.ProjectRef != nil {
			return nil, fmt.Errorf("projectRef is only valid for project-scoped robot permissions")
		}
		perms = append(perms, harborclient.RobotPermission{
			Kind:      perm.Kind,
			Namespace: namespace,
			Access:    access,
		})
	}
	return perms, nil
}

func robotLevelMatches(desired, current string) bool {
	return strings.EqualFold(desired, current)
}

func robotNameMatches(desired, current string) bool {
	if strings.EqualFold(desired, current) {
		return true
	}
	if strings.HasSuffix(strings.ToLower(current), "$"+strings.ToLower(desired)) {
		return true
	}
	if strings.HasSuffix(strings.ToLower(current), "+"+strings.ToLower(desired)) {
		return true
	}
	return false
}

func intPtrEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func shouldRotateRobot(cr *harborv1alpha1.Robot) bool {
	if cr.Status.ExpiresAt == nil {
		return false
	}
	return time.Now().After(cr.Status.ExpiresAt.Time)
}

func rotateRobotSecret(ctx context.Context, hc *harborclient.Client, robotID int) (string, error) {
	sec, err := hc.RefreshRobotSecret(ctx, robotID, "")
	if err != nil {
		return "", err
	}
	if sec.Secret == "" {
		return "", fmt.Errorf("harbor did not return a robot secret")
	}
	return sec.Secret, nil
}

func updateRobotExpiryStatus(cr *harborv1alpha1.Robot, expiresAt int) bool {
	var updated bool
	if expiresAt > 0 {
		exp := metav1.NewTime(time.Unix(int64(expiresAt), 0))
		if cr.Status.ExpiresAt == nil || !cr.Status.ExpiresAt.Equal(&exp) {
			cr.Status.ExpiresAt = &exp
			updated = true
		}
	} else if cr.Status.ExpiresAt != nil {
		cr.Status.ExpiresAt = nil
		updated = true
	}
	return updated
}

func (r *RobotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder, err := setupHarborBackedController(
		mgr,
		r.Options,
		&harborv1alpha1.Robot{},
		func() client.ObjectList { return &harborv1alpha1.RobotList{} },
		func(obj client.Object) *harborv1alpha1.HarborConnectionReference {
			return obj.(*harborv1alpha1.Robot).Spec.HarborConnectionRef
		},
		"robot",
	)
	if err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &harborv1alpha1.Robot{}, robotProjectRefIndex, func(raw client.Object) []string {
		robot := raw.(*harborv1alpha1.Robot)
		keys := make([]string, 0, len(robot.Spec.Permissions))
		seen := make(map[string]struct{}, len(robot.Spec.Permissions))
		for _, permission := range robot.Spec.Permissions {
			key := projectReferenceKey(robot.Namespace, permission.ProjectRef)
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
		return keys
	}); err != nil {
		return err
	}
	builder = builder.Watches(
		&harborv1alpha1.Project{},
		handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []reconcile.Request {
			return requestsForIndexedObjects(ctx, mgr, func() client.ObjectList { return &harborv1alpha1.RobotList{} }, robotProjectRefIndex, client.ObjectKeyFromObject(object).String())
		}),
	)
	return builder.Complete(r)
}
