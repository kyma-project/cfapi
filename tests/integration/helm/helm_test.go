package helm_test

import (
	"context"
	"errors"

	"github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/helm"
	"github.com/kyma-project/cfapi/controllers/installable"
	"github.com/kyma-project/cfapi/tests/integration/helm/fake"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("HelmChartInstallableIntegrationTest", func() {
	var (
		chartPath          string
		result             installable.Result
		valuesProvider     *fake.HelmValuesProvider
		installErr         error
		referencedSecret   *corev1.Secret
		helmChartInstaller *installable.HelmChart
	)

	BeforeEach(func() {
		valuesProvider = new(fake.HelmValuesProvider)
		valuesProvider.GetValuesReturns(map[string]any{}, nil)
		chartPath = "../../assets/dummy-chart"

		referencedSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace,
				Name:      "dummy-secret",
			},
			Data: map[string][]byte{
				"my-secret-key": []byte("my-secret-value"),
			},
		}
		Expect(k8sClient.Create(ctx, referencedSecret)).To(Succeed())
	})

	JustBeforeEach(func() {
		helmChartInstaller = installable.NewHelmChart(chartPath, testNamespace, "dummy-chart", valuesProvider, helm.NewClient())
		result, installErr = helmChartInstaller.Install(context.Background(), v1alpha1.InstallationConfig{}, new(fake.EventRecorder))
	})

	It("applies the chart", func() {
		Expect(installErr).NotTo(HaveOccurred())
		Expect(result.State).To(Equal(installable.ResultStateSuccess))

		Eventually(func(g Gomega) {
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNamespace,
					Name:      "dummy-configmap",
				},
			}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
			g.Expect(configMap.Data).To(Equal(map[string]string{
				"helm-value":    "dummy-value",
				"secret-values": "my-secret-value",
			}))
		}).Should(Succeed())
	})

	When("referenced secret does not exist", func() {
		BeforeEach(func() {
			Expect(k8sClient.Delete(ctx, referencedSecret)).To(Succeed())
		})

		It("it returns inprogress result", func() {
			Expect(installErr).NotTo(HaveOccurred())
			Expect(result.State).To(Equal(installable.ResultStateInProgress))
			Expect(result.Message).To(ContainSubstring("status unknown"))
		})
	})

	When("custom helm values are specified", func() {
		BeforeEach(func() {
			valuesProvider.GetValuesReturns(map[string]any{
				"configMapValue": "my-very-custom-value",
			}, nil)
		})

		It("uses them when applying the chart", func() {
			Expect(installErr).NotTo(HaveOccurred())
			Expect(result.State).To(Equal(installable.ResultStateSuccess))

			Eventually(func(g Gomega) {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "dummy-configmap",
					},
				}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
				g.Expect(configMap.Data).To(HaveKeyWithValue("helm-value", "my-very-custom-value"))
			}).Should(Succeed())
		})
	})

	When("the chart path is incorrect", func() {
		BeforeEach(func() {
			chartPath = "does-not-exist"
		})

		It("returns a failed result", func() {
			Expect(installErr).NotTo(HaveOccurred())
			Expect(result.State).To(Equal(installable.ResultStateFailed))
			Expect(result.Message).To(ContainSubstring("does-not-exist"))
		})
	})

	When("getting the custom values fails", func() {
		BeforeEach(func() {
			valuesProvider.GetValuesReturns(map[string]any{}, errors.New("values-err"))
		})

		It("returns a failed result", func() {
			Expect(installErr).NotTo(HaveOccurred())
			Expect(result.State).To(Equal(installable.ResultStateFailed))
			Expect(result.Message).To(ContainSubstring("values-err"))
		})
	})

	When("the chart is already installed", func() {
		JustBeforeEach(func() {
			Expect(installErr).NotTo(HaveOccurred())
			Expect(result.State).To(Equal(installable.ResultStateSuccess))

			Eventually(func(g Gomega) {
				Eventually(func(g Gomega) {
					configMap := &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testNamespace,
							Name:      "dummy-configmap",
						},
					}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
				}).Should(Succeed())
			}).Should(Succeed())
		})

		When("reinstalling the chart", func() {
			JustBeforeEach(func() {
				result, installErr = helmChartInstaller.Install(context.Background(), v1alpha1.InstallationConfig{}, new(fake.EventRecorder))
			})

			It("is noop", func() {
				Expect(installErr).NotTo(HaveOccurred())
				Expect(result.State).To(Equal(installable.ResultStateSuccess))

				Eventually(func(g Gomega) {
					configMap := &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testNamespace,
							Name:      "dummy-configmap",
						},
					}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
					g.Expect(configMap.Data).To(Equal(map[string]string{
						"helm-value":    "dummy-value",
						"secret-values": "my-secret-value",
					}))
				}).Should(Succeed())
			})
		})

		When("helm values change", func() {
			JustBeforeEach(func() {
				valuesProvider.GetValuesReturns(map[string]any{
					"configMapValue": "my-very-custom-value",
				}, nil)
				result, installErr = helmChartInstaller.Install(context.Background(), v1alpha1.InstallationConfig{}, new(fake.EventRecorder))
			})

			It("updates the helm resources with the new values", func() {
				Expect(installErr).NotTo(HaveOccurred())
				Expect(result.State).To(Equal(installable.ResultStateSuccess))

				Eventually(func(g Gomega) {
					configMap := &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testNamespace,
							Name:      "dummy-configmap",
						},
					}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
					g.Expect(configMap.Data).To(HaveKeyWithValue("helm-value", "my-very-custom-value"))
				}).Should(Succeed())
			})
		})
	})
})
