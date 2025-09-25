package create_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

* OIDC config: the resource described by module-data/oidc/oidc-uaa-experimental.tmpl exists and is properly rendered
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

var _ = Describe("Integration", func() {
	It("foos", func() {
		// time.Sleep(time.Minute)
		Expect(true).To(BeFalse())
	})
})
