package create_test

import (
	"crypto/tls"
	"net/http"

	"github.com/kyma-project/cfapi/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/*
* CFAPI resource:
  - Ready
  - has finalizer "sample.kyma-project.io/finalizer" (dow we need to rename it?)
  - Status.URL to be set and curlable (that is the korifi API url)
  - Status.State is set to Ready (v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1" -> v1alpha1.Ready)
  - Status.Conditions should be set to
  ```
  conditions:
  - lastTransitionTime: "2025-09-25T10:56:06Z"
    message: installation is ready and resources can be used
    observedGeneration: 1
    reason: Ready
    status: "True"
    type: Installation
  ```

* Ensure Gateway API is installed by checking the exsitence of a gwapi CRD
* Ensure PILOT_ENABLE_ALPHA_GATEWAY_API is set on the istiod deployment
* Ensure the resource from module-data/envoy-filter/empty-envoy-filter.yaml exists
* Ensure cf and korifi namesapces exist
* Ensure secret `cfapi-app-registry` exists in both `cf` and `korifi` namespaces.The secret:
   - should be of type `kubernetes.io/dockerconfigjson` (is there a constant?)
   - should have a key  `..dockerconfigjson` that can be unmarshalled into DockerRegistryConfig; check whether the `DockerRegistryConfig` attributes match the actual docker registry
* Certificates
  - check the certificates from ./module-data/certificates/certificates.tmpl exist and are properly rendered
  - check that they eventually become Ready
  - check that they result into expected secrets
* Check kpack is installed
  - the `kpack/kpack-controller` deployment should be running (i.e. its `Status.ReadyReplicas` is > 0)
* Korifi:
- check all korifi deployments are running:
```
korifi                                         korifi-api-deployment
korifi                                         korifi-controllers-controller-manager
korifi-gateway                                 korifi-istio
korifi                                         korifi-job-task-runner-controller-manager
korifi                                         korifi-kpack-image-builder-controller-manager
korifi                                         korifi-statefulset-runner-controller-manager
```

  - check korifi deployments config maps for the values that are being set dynamically in the `deployKorifi` function
  *DNS entries: check whether entries from module-data/dns-entries/dns-entries.tmpl exist and are properly reendered
  * Clusteradmin
    - the test should create a cluster admin user before applying the crd ( a role binding to the cluster-admin role)
	- check whether a role binding `cf/cfapi-admins-binding` that bind the admin user to the `korifi-controllers-admin` cluster role
* BTP Service broker
  - check the deployment is running

*/

var _ = FDescribe("Integration", func() {
	Describe("CFAPI resource", func() {
		var cfAPI *v1alpha1.CFAPI

		BeforeEach(func() {
			cfAPI = &v1alpha1.CFAPI{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "cfapi-system",
					Name:      cfAPIName,
				},
			}
		})

		It("creates CFAPI resource with ready status", func() {
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())

				condition := meta.FindStatusCondition(cfAPI.Status.Conditions, v1alpha1.ConditionTypeInstallation)
				g.Expect(condition).ShouldNot(BeNil())
				g.Expect(condition.Reason).To(Equal(v1alpha1.ConditionReasonReady))
				g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())
		})

		It("sets the finalizer on the resource", func() {
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
				g.Expect(cfAPI.Finalizers).To(ConsistOf("sample.kyma-project.io/finalizer"))
			}).Should(Succeed())
		})

		Describe("cfapi url", func() {
			BeforeEach(func() {
				Eventually(func(g Gomega) {
					gwService := &corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "istio-system",
							Name:      "istio-ingressgateway",
						},
					}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(gwService), gwService)).To(Succeed())

					modifiedGwService := gwService.DeepCopy()

					ports := []corev1.ServicePort{}
					for _, port := range modifiedGwService.Spec.Ports {
						if port.Name == "https" {
							port.NodePort = 32443
							port.Port = 443
							port.Protocol = corev1.ProtocolTCP
							port.TargetPort.IntVal = 8443
						}

						ports = append(ports, port)
					}
					modifiedGwService.Spec.Ports = ports
					g.Expect(k8sClient.Patch(ctx, modifiedGwService, client.MergeFrom(gwService))).To(Succeed())
				}).Should(Succeed())
			})

			FIt("sets usable cfapi url", func() {
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
					g.Expect(cfAPI.Status.URL).NotTo(BeEmpty())

					http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
					resp, err := http.Get(cfAPI.Status.URL)
					g.Expect(err).NotTo(HaveOccurred())
					resp.Body.Close()
					g.Expect(resp.StatusCode).To(Equal(http.StatusOK))
				}, "5m").Should(Succeed())
			})
		})
	})
	// It("foos", func() {
	// 	// time.Sleep(time.Minute)
	// 	Expect(true).To(BeFalse())
	// })
})
