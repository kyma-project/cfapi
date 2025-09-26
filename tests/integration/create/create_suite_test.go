package create_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kubectl/pkg/scheme"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme.Scheme))
	utilruntime.Must(controllers.AddToScheme(scheme.Scheme))
}

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Create Integration Suite")
}

var (
	ctx       context.Context
	k8sClient client.Client
	cfApiName string
)

type sharedSetupData struct {
	CfAPIName string `json:"cf_api_name"`
}

func commonTestSetup() {
	SetDefaultEventuallyTimeout(4 * time.Minute)
	SetDefaultEventuallyPollingInterval(2 * time.Second)

	ctx = context.Background()
	k8sClient = createK8sClient()
}

var _ = SynchronizedBeforeSuite(func() []byte {
	commonTestSetup()

	cfAPI := &v1alpha1.CFAPI{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "cfapi-system",
			Name:      uuid.NewString(),
		},
		Spec: v1alpha1.CFAPISpec{
			RootNamespace: "cf",
		},
	}
	Expect(k8sClient.Create(ctx, cfAPI)).To(Succeed())

	sharedData := sharedSetupData{
		CfAPIName: cfAPI.Name,
	}

	bs, err := json.Marshal(sharedData)
	Expect(err).NotTo(HaveOccurred())

	return bs
}, func(bs []byte) {
	commonTestSetup()

	var sharedSetup sharedSetupData
	err := json.Unmarshal(bs, &sharedSetup)
	Expect(err).NotTo(HaveOccurred())

	cfApiName = sharedSetup.CfAPIName
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	Expect(k8sClient.Delete(ctx, &v1alpha1.CFAPI{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "cfapi-system",
			Name:      cfApiName,
		},
	})).To(Succeed())

	/* Cleanup
	- Delete the cf namespace
	- Delete the cfapi-system/btp-service-broker helm chart
	- Delete the korifi/korifi helm chart
	- Delete the korifi namespace
	- Gateway API:
	  - vendor gwapi yaml and make the operator Dockerfile package it from the vendor directory instead of pulling it form github
	  - in the test SynchronizedAfterSuite, delete the gwapi yaml (from the vendor directory)
	- Kpack:
	  - vendor kpack yaml and make the operator Dockerfile package it from the vendor directory instead of pulling it form github
	  - in the test SynchronizedAfterSuite, delete the kpack yaml (from the vendor directory)
	- Delete the module-data/envoy-filter/empty-envoy-filter.yaml resource
	*/
})

func createK8sClient() client.Client {
	config, err := controllerruntime.GetConfig()
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err := client.New(config, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	return k8sClient
}
