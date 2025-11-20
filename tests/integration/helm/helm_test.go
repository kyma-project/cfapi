package helm_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	golog "log"

	"github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/helm"
	"github.com/kyma-project/cfapi/controllers/installable"
	"github.com/kyma-project/cfapi/tests/helpers"
	"github.com/kyma-project/cfapi/tests/integration/helm/fake"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("HelmChartInstallableIntegrationTest", func() {
	var (
		chartPath      string
		result         installable.Result
		valuesProvider *fake.HelmValuesProvider

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
		helpers.EnsureCreate(k8sClient, referencedSecret)
	})

	Describe("Install", func() {
		var installErr error

		JustBeforeEach(func() {
			helmChartInstaller = installable.NewHelmChart(chartPath, testNamespace, "dummy-chart", valuesProvider, helm.NewClient())
			result, installErr = helmChartInstaller.Install(ctx, v1alpha1.InstallationConfig{}, new(fake.EventRecorder))
		})

		It("applies the chart", func() {
			Expect(installErr).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				installedReleases := listReleases("dummy-chart")
				g.Expect(installedReleases).To(HaveLen(1))
				g.Expect(installedReleases[0].Info.Status).To(Equal(release.StatusDeployed))

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
				helpers.EnsureDelete(k8sClient, referencedSecret)
			})

			It("it returns in progress result", func() {
				Expect(installErr).NotTo(HaveOccurred())
				Expect(result.State).To(Equal(installable.ResultStateInProgress))
				Expect(result.Message).To(SatisfyAll(ContainSubstring("status unknown"), ContainSubstring("lookup")))
			})

			It("does not deploy the chart (as helm templating fails)", func() {
				Consistently(func(g Gomega) {
					g.Expect(listReleases("dummy-chart")).To(BeEmpty())
				}).Should(Succeed())
			})
		})

		When("custom helm values are specified", func() {
			BeforeEach(func() {
				valuesProvider.GetValuesReturns(map[string]any{
					"configMapValue": "my-very-custom-value",
				}, nil)
			})

			It("uses them when applying the chart", func() {
				Eventually(func(g Gomega) {
					installedReleases := listReleases("dummy-chart")
					g.Expect(installedReleases).To(HaveLen(1))
					g.Expect(installedReleases[0].Info.Status).To(Equal(release.StatusDeployed))

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

			It("returns in progress result", func() {
				Expect(installErr).NotTo(HaveOccurred())
				Expect(result.State).To(Equal(installable.ResultStateInProgress))
			})
		})

		When("the chart is already installed", func() {
			JustBeforeEach(func() {
				Expect(installErr).NotTo(HaveOccurred())

				Eventually(func(g Gomega) {
					installedReleases := listReleases("dummy-chart")
					g.Expect(installedReleases).To(HaveLen(1))
					g.Expect(installedReleases[0].Info.Status).To(Equal(release.StatusDeployed))
				}).Should(Succeed())
			})

			When("reinstalling the chart", func() {
				JustBeforeEach(func() {
					result, installErr = helmChartInstaller.Install(context.Background(), v1alpha1.InstallationConfig{}, new(fake.EventRecorder))
				})

				It("is noop", func() {
					Expect(installErr).NotTo(HaveOccurred())
					Expect(result.State).To(Equal(installable.ResultStateSuccess))

					installedReleases := listReleases("dummy-chart")
					Expect(installedReleases).To(HaveLen(1))
					Expect(installedReleases[0].Info.Status).To(Equal(release.StatusDeployed))
				})
			})

			When("reinstalling the chart with new values", func() {
				JustBeforeEach(func() {
					valuesProvider.GetValuesReturns(map[string]any{
						"configMapValue": "my-very-custom-value",
					}, nil)
					time.Sleep(time.Second)
					result, installErr = helmChartInstaller.Install(context.Background(), v1alpha1.InstallationConfig{}, new(fake.EventRecorder))
				})

				It("updates the helm resources with the new values", func() {
					Expect(installErr).NotTo(HaveOccurred())
					Expect(result.State).To(Equal(installable.ResultStateSuccess))

					Eventually(func(g Gomega) {
						installedReleases := listReleases("dummy-chart")
						g.Expect(installedReleases).To(HaveLen(2))
						g.Expect(installedReleases[0].Info.Status).To(Equal(release.StatusSuperseded))
						g.Expect(installedReleases[1].Info.Status).To(Equal(release.StatusDeployed))

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

			When("upgrading to a newer chart version", func() {
				JustBeforeEach(func() {
					helmChartInstaller = installable.NewHelmChart("../../assets/dummy-chart-v2", testNamespace, "dummy-chart", valuesProvider, helm.NewClient())
					result, installErr = helmChartInstaller.Install(context.Background(), v1alpha1.InstallationConfig{}, new(fake.EventRecorder))
				})

				It("upgrades to the new helm version", func() {
					Expect(installErr).NotTo(HaveOccurred())
					Expect(result).To(Equal(installable.Result{}))

					Eventually(func(g Gomega) {
						installedReleases := listReleases("dummy-chart")
						g.Expect(installedReleases).To(HaveLen(2))
						g.Expect(installedReleases[0].Info.Status).To(Equal(release.StatusSuperseded))
						g.Expect(installedReleases[1].Info.Status).To(Equal(release.StatusDeployed))

						oldConfigMap := &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: testNamespace,
								Name:      "dummy-configmap",
							},
						}
						g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(oldConfigMap), oldConfigMap))).To(BeTrue())

						newConfigMap := &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: testNamespace,
								Name:      "dummy-configmap-v2",
							},
						}
						g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(newConfigMap), newConfigMap)).To(Succeed())
						g.Expect(newConfigMap.Data).To(HaveKeyWithValue("v2-key", "v2-value"))
					}).Should(Succeed())
				})
			})
		})
	})

	Describe("Uninstall", func() {
		var uninstallErr error

		JustBeforeEach(func() {
			helmChartInstaller = installable.NewHelmChart(chartPath, testNamespace, "dummy-chart", valuesProvider, helm.NewClient())
			result, uninstallErr = helmChartInstaller.Uninstall(ctx, v1alpha1.InstallationConfig{}, new(fake.EventRecorder))
		})

		It("succeeds", func() {
			Expect(uninstallErr).NotTo(HaveOccurred())
		})

		When("the chart is installed", func() {
			BeforeEach(func() {
				installChart(chartPath, testNamespace, "dummy-chart")
			})

			It("uninstalls the release", func() {
				Expect(uninstallErr).NotTo(HaveOccurred())
				Eventually(func(g Gomega) {
					g.Expect(listReleases("dummy-chart")).To(BeEmpty())
				}).Should(Succeed())
			})
		})
	})
})

func listReleases(releaseName string) []*release.Release {
	actionConfig, err := newHelmActionConfig(testNamespace)
	Expect(err).NotTo(HaveOccurred())

	historyClient := action.NewHistory(actionConfig)
	versions, err := historyClient.Run(releaseName)
	if err != nil {
		if err == driver.ErrReleaseNotFound {
			return nil
		}
		Expect(err).NotTo(HaveOccurred())
	}

	return versions
}

func installChart(chartPath, releaseNamespace, releaseName string) {
	chart, err := loader.Load(chartPath)
	Expect(err).NotTo(HaveOccurred())

	actionConfig, err := newHelmActionConfig(testNamespace)
	Expect(err).NotTo(HaveOccurred())

	installAction := action.NewInstall(actionConfig)
	installAction.Namespace = releaseNamespace
	installAction.ReleaseName = releaseName

	_, err = installAction.Run(chart, nil)
	Expect(err).NotTo(HaveOccurred())
}

func newHelmActionConfig(releaseNamespace string) (*action.Configuration, error) {
	helmSettings := cli.New()
	actionConfig := new(action.Configuration)
	err := actionConfig.Init(helmSettings.RESTClientGetter(), releaseNamespace, "secret", golog.Printf)
	if err != nil {
		return nil, fmt.Errorf("failed to init helm action config: %w", err)
	}

	return actionConfig, nil
}
