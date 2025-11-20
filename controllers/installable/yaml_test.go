package installable_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/installable"
	"github.com/kyma-project/cfapi/tests/helpers"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Yaml", func() {
	Describe("Install File", func() {
		var (
			yamlContent string

			installResult installable.Result
			installErr    error
		)

		BeforeEach(func() {
			yamlContent = ""
		})

		JustBeforeEach(func() {
			yamlFile, err := os.CreateTemp("", "")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				Expect(os.RemoveAll(yamlFile.Name())).To(Succeed())
			})

			_, err = io.WriteString(yamlFile, yamlContent)
			Expect(err).NotTo(HaveOccurred())

			installResult, installErr = installable.NewYaml(adminClient, yamlFile.Name(), "test-file").
				Install(ctx, v1alpha1.InstallationConfig{}, eventRecorder)
		})

		It("succeeds for empty yaml", func() {
			Expect(installErr).NotTo(HaveOccurred())
			Expect(installResult.State).To(Equal(installable.ResultStateSuccess))
		})

		When("the yaml contains objects", func() {
			BeforeEach(func() {
				yamlContent = fmt.Sprintf(
					`apiVersion: v1
kind: ConfigMap
metadata:
  name: map1
  namespace: %s
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: map2
  namespace: %s`, testNamespace, testNamespace)
			})

			It("creates them", func() {
				Expect(installErr).NotTo(HaveOccurred())
				Expect(installResult.State).To(Equal(installable.ResultStateSuccess))
				Expect(adminClient.Get(ctx, client.ObjectKey{Name: "map1", Namespace: testNamespace}, &corev1.ConfigMap{})).To(Succeed())
				Expect(adminClient.Get(ctx, client.ObjectKey{Name: "map2", Namespace: testNamespace}, &corev1.ConfigMap{})).To(Succeed())
			})

			When("an object already exists", func() {
				BeforeEach(func() {
					helpers.EnsureCreate(adminClient, &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testNamespace,
							Name:      "map1",
							Labels: map[string]string{
								"foo": "bar",
							},
						},
					})
				})

				It("updates it", func() {
					Expect(installErr).NotTo(HaveOccurred())
					Expect(installResult.State).To(Equal(installable.ResultStateSuccess))

					m := &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testNamespace,
							Name:      "map1",
						},
					}
					Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(m), m)).To(Succeed())
					Expect(m.Labels).To(BeEmpty())
				})
			})
		})

		When("the yaml is invalid", func() {
			BeforeEach(func() {
				yamlContent = "invalid-yaml"
			})

			It("returns a failed result", func() {
				Expect(installErr).NotTo(HaveOccurred())
				Expect(installResult.State).To(Equal(installable.ResultStateFailed))
			})
		})

		When("create fails", func() {
			BeforeEach(func() {
				yamlContent = fmt.Sprintf(
					`apiVersion: v1
kind: NotExistingType
metadata:
  name: whatever
  namespace: %s`, testNamespace)
			})

			It("returns an error", func() {
				Expect(installErr).To(MatchError(ContainSubstring("failed to create")))
			})
		})

		When("the yaml file does not exist", func() {
			var (
				installResult1 installable.Result
				installErr1    error
			)
			BeforeEach(func() {
				installResult1, installErr1 = installable.NewYaml(adminClient, "file-does-not-exist", "").
					Install(ctx, v1alpha1.InstallationConfig{}, eventRecorder)
			})

			It("returns an error", func() {
				Expect(installErr1).NotTo(HaveOccurred())
				Expect(installResult1.State).To(Equal(installable.ResultStateFailed))
			})
		})
	})

	Describe("Install YamlGlob", func() {
		var (
			yamlGlob   *installable.Yaml
			result     installable.Result
			installErr error
		)

		BeforeEach(func() {
			tmpDir, err := os.MkdirTemp("", "")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				Expect(os.RemoveAll(tmpDir)).To(Succeed())
			})

			first, err := os.CreateTemp(tmpDir, "first.yaml")
			Expect(err).NotTo(HaveOccurred())
			defer first.Close()

			_, err = io.WriteString(first, fmt.Sprintf(
				`apiVersion: v1
kind: ConfigMap
metadata:
  name: map1
  namespace: %s
`, testNamespace))
			Expect(err).NotTo(HaveOccurred())

			second, err := os.CreateTemp(tmpDir, "second.yaml")
			Expect(err).NotTo(HaveOccurred())
			defer second.Close()

			_, err = io.WriteString(second, fmt.Sprintf(
				`apiVersion: v1
kind: ConfigMap
metadata:
  name: map2
  namespace: %s
`, testNamespace))
			Expect(err).NotTo(HaveOccurred())

			yamlGlob = installable.NewYaml(adminClient, filepath.Join(tmpDir, "f*.yaml*"), "glob")
		})

		JustBeforeEach(func() {
			result, installErr = yamlGlob.Install(ctx, v1alpha1.InstallationConfig{}, eventRecorder)
		})

		It("applies matching yamls", func() {
			Expect(installErr).NotTo(HaveOccurred())
			Expect(result.State).To(Equal(installable.ResultStateSuccess))

			expectedMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNamespace,
					Name:      "map1",
				},
			}
			Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(expectedMap), expectedMap)).To(Succeed())
		})
	})

	Describe("Uninstall", func() {
		var (
			yamlContent string

			uninstallResult installable.Result
			uninstallErr    error
		)

		BeforeEach(func() {
			yamlContent = ""
		})

		JustBeforeEach(func() {
			yamlFile, err := os.CreateTemp("", "")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				Expect(os.RemoveAll(yamlFile.Name())).To(Succeed())
			})

			_, err = io.WriteString(yamlFile, yamlContent)
			Expect(err).NotTo(HaveOccurred())

			uninstallResult, uninstallErr = installable.NewYaml(adminClient, yamlFile.Name(), "test-file").
				Uninstall(ctx, v1alpha1.InstallationConfig{}, eventRecorder)
		})

		It("succeeds for empty yaml", func() {
			Expect(uninstallErr).NotTo(HaveOccurred())
			Expect(uninstallResult.State).To(Equal(installable.ResultStateSuccess))
		})

		When("the yaml contains objects", func() {
			BeforeEach(func() {
				yamlContent = fmt.Sprintf(
					`apiVersion: v1
kind: ConfigMap
metadata:
  name: map1
  namespace: %s
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: map2
  namespace: %s`, testNamespace, testNamespace)
			})

			It("retuns success", func() {
				Expect(uninstallErr).NotTo(HaveOccurred())
				Expect(uninstallResult.State).To(Equal(installable.ResultStateSuccess))
			})

			When("the objects to be deleted exist", func() {
				BeforeEach(func() {
					helpers.EnsureCreate(adminClient, &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testNamespace,
							Name:      "map1",
						},
					})
					helpers.EnsureCreate(adminClient, &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testNamespace,
							Name:      "map2",
						},
					})
				})

				It("deletes them and returns in progress result", func() {
					Expect(uninstallErr).NotTo(HaveOccurred())
					Expect(uninstallResult.State).To(Equal(installable.ResultStateInProgress))

					err := adminClient.Get(ctx, client.ObjectKey{Name: "map1", Namespace: testNamespace}, &corev1.ConfigMap{})
					Expect(k8serrors.IsNotFound(err)).To(BeTrue())

					err = adminClient.Get(ctx, client.ObjectKey{Name: "map2", Namespace: testNamespace}, &corev1.ConfigMap{})
					Expect(k8serrors.IsNotFound(err)).To(BeTrue())
				})
			})
		})

		When("the yaml is invalid", func() {
			BeforeEach(func() {
				yamlContent = "invalid-yaml"
			})

			It("returns a failed result", func() {
				Expect(uninstallErr).NotTo(HaveOccurred())
				Expect(uninstallResult.State).To(Equal(installable.ResultStateFailed))
			})
		})

		When("the yaml file does not exist", func() {
			var (
				installResult1 installable.Result
				installErr1    error
			)
			BeforeEach(func() {
				installResult1, installErr1 = installable.NewYaml(adminClient, "file-does-not-exist", "").
					Install(ctx, v1alpha1.InstallationConfig{}, eventRecorder)
			})

			It("returns an error", func() {
				Expect(installErr1).NotTo(HaveOccurred())
				Expect(installResult1.State).To(Equal(installable.ResultStateFailed))
			})
		})
	})
})
