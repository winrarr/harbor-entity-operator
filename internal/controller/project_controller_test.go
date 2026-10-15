/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

func TestProjectNeedsUpdateUsesSinglePublicField(t *testing.T) {
	tests := []struct {
		name       string
		public     bool
		harbor     string
		wantUpdate bool
	}{
		{name: "public project already matches", public: true, harbor: "true"},
		{name: "private project already matches", harbor: "false"},
		{name: "visibility drift is detected", public: true, harbor: "false", wantUpdate: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			desired := harborclient.CreateProjectRequest{
				Public:   test.public,
				Metadata: harborclient.ProjectMetadata{Public: strconv.FormatBool(test.public)},
			}
			current := harborclient.Project{
				Metadata: harborclient.ProjectMetadata{Public: test.harbor},
			}

			if got := projectNeedsUpdate(desired, current); got != test.wantUpdate {
				t.Fatalf("projectNeedsUpdate() = %t, want %t", got, test.wantUpdate)
			}
		})
	}
}

func TestProjectNeedsUpdateLeavesOmittedCVEAllowlistUnmanaged(t *testing.T) {
	desired := harborclient.CreateProjectRequest{
		ProjectName: "project",
		Metadata:    harborclient.ProjectMetadata{Public: "false"},
	}
	current := harborclient.Project{
		Name:     "project",
		Metadata: harborclient.ProjectMetadata{Public: "false"},
		CVEAllowlist: harborclient.CVEAllowlist{
			ID:        7,
			ProjectID: 42,
			Items:     []harborclient.CVEAllowlistItem{{CveID: "CVE-2024-12345"}},
		},
	}

	if projectNeedsUpdate(desired, current) {
		t.Fatal("projectNeedsUpdate() requested an update for an omitted CVE allowlist")
	}
}

func TestProjectNeedsUpdateClearsExplicitEmptyCVEAllowlist(t *testing.T) {
	desired := harborclient.CreateProjectRequest{
		ProjectName:  "project",
		Metadata:     harborclient.ProjectMetadata{Public: "false"},
		CVEAllowlist: &harborclient.CVEAllowlist{Items: []harborclient.CVEAllowlistItem{}},
	}
	current := harborclient.Project{
		Name:     "project",
		Metadata: harborclient.ProjectMetadata{Public: "false"},
		CVEAllowlist: harborclient.CVEAllowlist{
			Items: []harborclient.CVEAllowlistItem{{CveID: "CVE-2024-12345"}},
		},
	}

	if !projectNeedsUpdate(desired, current) {
		t.Fatal("projectNeedsUpdate() ignored an explicit request to clear the CVE allowlist")
	}
}

func TestProjectNeedsUpdateIgnoresHarborManagedCVEAllowlistIdentity(t *testing.T) {
	desired := harborclient.CreateProjectRequest{
		ProjectName: "project",
		Metadata:    harborclient.ProjectMetadata{Public: "false"},
	}
	current := harborclient.Project{
		Name:     "project",
		Metadata: harborclient.ProjectMetadata{Public: "false"},
		CVEAllowlist: harborclient.CVEAllowlist{
			ID:        7,
			ProjectID: 42,
		},
	}

	if got := projectNeedsUpdate(desired, current); got {
		t.Fatal("projectNeedsUpdate() requested an update for Harbor-generated CVE allowlist IDs")
	}
}

func TestProjectNeedsUpdateIgnoresHarborDefaultOwnerWhenUnspecified(t *testing.T) {
	desired := harborclient.CreateProjectRequest{
		ProjectName: "project",
		Metadata:    harborclient.ProjectMetadata{Public: "false"},
	}
	current := harborclient.Project{
		Name:      "project",
		OwnerName: "admin",
		Metadata:  harborclient.ProjectMetadata{Public: "false"},
	}

	if got := projectNeedsUpdate(desired, current); got {
		t.Fatal("projectNeedsUpdate() requested an update for an unspecified Harbor-defaulted owner")
	}
}

func TestProjectNeedsUpdateIgnoresUnspecifiedProjectMetadataDefaults(t *testing.T) {
	retentionID := "3"
	desired := harborclient.CreateProjectRequest{
		ProjectName: "project",
		Public:      true,
		Metadata:    harborclient.ProjectMetadata{Public: "true"},
	}
	current := harborclient.Project{
		Name:      "project",
		OwnerName: "admin",
		Metadata: harborclient.ProjectMetadata{
			Public:      "true",
			RetentionID: &retentionID,
		},
	}

	if got := projectNeedsUpdate(desired, current); got {
		t.Fatal("projectNeedsUpdate() requested an update for unspecified Harbor-defaulted metadata")
	}
}

func TestProjectNeedsUpdateCanClearExplicitlyEmptyMetadata(t *testing.T) {
	empty := ""
	currentPattern := "release/**"
	desired := harborclient.CreateProjectRequest{
		ProjectName: "project",
		Metadata: harborclient.ProjectMetadata{
			Public:                  "false",
			ProxyCacheFilterPattern: &empty,
		},
	}
	current := harborclient.Project{
		Name: "project",
		Metadata: harborclient.ProjectMetadata{
			Public:                  "false",
			ProxyCacheFilterPattern: &currentPattern,
		},
	}

	if !projectNeedsUpdate(desired, current) {
		t.Fatal("projectNeedsUpdate() ignored an explicit empty value to clear metadata")
	}
	current.Metadata.ProxyCacheFilterPattern = nil
	if projectNeedsUpdate(desired, current) {
		t.Fatal("projectNeedsUpdate() requested an update when Harbor omitted a cleared empty metadata value")
	}
}

var _ = Describe("Project Controller", func() {
	const projectsPath = "/api/v2.0/projects"

	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		project := &harborv1alpha1.Project{}
		var server *httptest.Server

		BeforeEach(func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, pass, ok := r.BasicAuth()
				if !ok || user != testAdminUser || pass != testPassword {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if r.Method == http.MethodPost && r.URL.Path == projectsPath {
					w.Header().Set("Location", "/api/v2.0/projects/42")
					w.WriteHeader(http.StatusCreated)
					return
				}
				http.NotFound(w, r)
			}))

			Expect(createPasswordSecret(ctx, k8sClient, "harbor-admin", testPassword)).To(Succeed())
			Expect(createHarborConnection(ctx, k8sClient, "harbor-conn", server.URL, "harbor-admin")).To(Succeed())

			By("creating the custom resource for the Kind Project")
			resource := &harborv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: harborv1alpha1.ProjectSpec{
					HarborSpecBase: harborv1alpha1.HarborSpecBase{
						HarborConnectionRef: &harborv1alpha1.HarborConnectionReference{Name: "harbor-conn"},
					},
					Public: false,
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			server.Close()
			resource := &harborv1alpha1.Project{}
			_ = k8sClient.Get(ctx, typeNamespacedName, resource)

			By("Cleanup the specific resource instance Project")
			_ = k8sClient.Delete(ctx, resource)

			conn := &harborv1alpha1.HarborConnection{}
			_ = k8sClient.Get(ctx, types.NamespacedName{Name: "harbor-conn", Namespace: "default"}, conn)
			_ = k8sClient.Delete(ctx, conn)
			secret := &corev1.Secret{}
			_ = k8sClient.Get(ctx, types.NamespacedName{Name: "harbor-admin", Namespace: "default"}, secret)
			_ = k8sClient.Delete(ctx, secret)
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &ProjectReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, typeNamespacedName, project)).To(Succeed())
			Expect(project.Status.HarborProjectID).To(Equal(42))
		})
	})

	Context("When adopting an existing project", func() {
		const resourceName = "adopted-project"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		project := &harborv1alpha1.Project{}
		var server *httptest.Server

		BeforeEach(func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, pass, ok := r.BasicAuth()
				if !ok || user != testAdminUser || pass != testPassword {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if r.Method == http.MethodGet && r.URL.Path == projectsPath {
					if r.URL.Query().Get("page") != "1" ||
						r.URL.Query().Get("page_size") != "100" ||
						r.URL.Query().Get("q") != "name="+resourceName {
						http.Error(w, "missing project name query", http.StatusBadRequest)
						return
					}
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`[{"project_id":42,"name":"adopted-project","metadata":{"public":"false"}}]`))
					return
				}
				if r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/projects/42" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"project_id":42,"name":"adopted-project","metadata":{"public":"false"}}`))
					return
				}
				if r.Method == http.MethodPost && r.URL.Path == projectsPath {
					w.WriteHeader(http.StatusConflict)
					return
				}
				http.NotFound(w, r)
			}))

			Expect(createPasswordSecret(ctx, k8sClient, "harbor-admin", testPassword)).To(Succeed())
			Expect(createHarborConnection(ctx, k8sClient, "harbor-conn", server.URL, "harbor-admin")).To(Succeed())

			resource := &harborv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: harborv1alpha1.ProjectSpec{
					HarborSpecBase: harborv1alpha1.HarborSpecBase{
						HarborConnectionRef: &harborv1alpha1.HarborConnectionReference{Name: "harbor-conn"},
					},
					CreationPolicy: harborv1alpha1.CreationPolicyCreateOrAdopt,
					Public:         false,
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			server.Close()
			resource := &harborv1alpha1.Project{}
			if k8sClient.Get(ctx, typeNamespacedName, resource) == nil {
				resource.Finalizers = nil
				_ = k8sClient.Update(ctx, resource)
				_ = k8sClient.Delete(ctx, resource)
			}

			conn := &harborv1alpha1.HarborConnection{}
			_ = k8sClient.Get(ctx, types.NamespacedName{Name: "harbor-conn", Namespace: "default"}, conn)
			_ = k8sClient.Delete(ctx, conn)
			secret := &corev1.Secret{}
			_ = k8sClient.Get(ctx, types.NamespacedName{Name: "harbor-admin", Namespace: "default"}, secret)
			_ = k8sClient.Delete(ctx, secret)
		})

		It("finds the existing Harbor project by name before creating", func() {
			controllerReconciler := &ProjectReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, typeNamespacedName, project)).To(Succeed())
			Expect(project.Status.HarborProjectID).To(Equal(42))
		})

		It("adopts the existing project with creation policy Adopt", func() {
			Expect(k8sClient.Get(ctx, typeNamespacedName, project)).To(Succeed())
			project.Spec.CreationPolicy = harborv1alpha1.CreationPolicyAdopt
			Expect(k8sClient.Update(ctx, project)).To(Succeed())

			controllerReconciler := &ProjectReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, typeNamespacedName, project)).To(Succeed())
			Expect(project.Status.HarborProjectID).To(Equal(42))
		})

		It("uses the operator default when creation policy is omitted", func() {
			options, err := NewOperatorOptions(OperatorConfig{
				DefaultCreationPolicy: harborv1alpha1.CreationPolicyAdopt,
				HarborRequestTimeout:  30 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, typeNamespacedName, project)).To(Succeed())
			project.Spec.CreationPolicy = ""
			Expect(k8sClient.Update(ctx, project)).To(Succeed())

			controllerReconciler := &ProjectReconciler{
				Client:  k8sClient,
				Scheme:  k8sClient.Scheme(),
				Options: options,
			}
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, typeNamespacedName, project)).To(Succeed())
			Expect(project.Spec.CreationPolicy).To(BeEmpty())
			Expect(project.Status.HarborProjectID).To(Equal(42))
		})
	})

	Context("When Adopt cannot find an existing project", func() {
		const resourceName = "missing-adopted-project"

		ctx := context.Background()
		typeNamespacedName := types.NamespacedName{Name: resourceName, Namespace: "default"}
		var server *httptest.Server
		createRequested := false

		BeforeEach(func() {
			createRequested = false
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, pass, ok := r.BasicAuth()
				if !ok || user != testAdminUser || pass != testPassword {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if r.Method == http.MethodGet && r.URL.Path == projectsPath {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`[]`))
					return
				}
				if r.Method == http.MethodPost && r.URL.Path == projectsPath {
					createRequested = true
					w.WriteHeader(http.StatusCreated)
					return
				}
				http.NotFound(w, r)
			}))

			Expect(createPasswordSecret(ctx, k8sClient, "harbor-admin-adopt", testPassword)).To(Succeed())
			Expect(createHarborConnection(ctx, k8sClient, "harbor-conn-adopt", server.URL, "harbor-admin-adopt")).To(Succeed())
			resource := &harborv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: "default"},
				Spec: harborv1alpha1.ProjectSpec{
					HarborSpecBase: harborv1alpha1.HarborSpecBase{
						HarborConnectionRef: &harborv1alpha1.HarborConnectionReference{Name: "harbor-conn-adopt"},
					},
					CreationPolicy: harborv1alpha1.CreationPolicyAdopt,
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			server.Close()
			resource := &harborv1alpha1.Project{}
			_ = k8sClient.Get(ctx, typeNamespacedName, resource)
			_ = k8sClient.Delete(ctx, resource)
			conn := &harborv1alpha1.HarborConnection{}
			_ = k8sClient.Get(ctx, types.NamespacedName{Name: "harbor-conn-adopt", Namespace: "default"}, conn)
			_ = k8sClient.Delete(ctx, conn)
			secret := &corev1.Secret{}
			_ = k8sClient.Get(ctx, types.NamespacedName{Name: "harbor-admin-adopt", Namespace: "default"}, secret)
			_ = k8sClient.Delete(ctx, secret)
		})

		It("reports an error without creating a project", func() {
			controllerReconciler := &ProjectReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).To(MatchError(ContainSubstring("requires an existing Harbor resource to adopt")))
			Expect(createRequested).To(BeFalse())
		})
	})
})
