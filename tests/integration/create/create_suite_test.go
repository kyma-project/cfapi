package create_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	golog "log"

	"github.com/google/uuid"
	"github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers"
	"github.com/kyma-project/cfapi/tests/integration/helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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
	cfAPIName string
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

	Eventually(func(g Gomega) {
		gwService := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "korifi-gateway",
				Name:      "korifi-istio",
			},
		}
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(gwService), gwService)).To(Succeed())

		modifiedGwService := gwService.DeepCopy()

		ports := []corev1.ServicePort{}
		for _, port := range modifiedGwService.Spec.Ports {
			if port.Name == "https-api" {
				port.NodePort = 33443
				port.Port = 443
				port.Protocol = corev1.ProtocolTCP
				port.TargetPort.IntVal = 8443
			}

			ports = append(ports, port)
		}
		modifiedGwService.Spec.Ports = ports
		g.Expect(k8sClient.Patch(ctx, modifiedGwService, client.MergeFrom(gwService))).To(Succeed())
	}).Should(Succeed())

	return bs
}, func(bs []byte) {
	commonTestSetup()

	var sharedSetup sharedSetupData
	err := json.Unmarshal(bs, &sharedSetup)
	Expect(err).NotTo(HaveOccurred())

	cfAPIName = sharedSetup.CfAPIName
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	Expect(k8sClient.Delete(ctx, &v1alpha1.CFAPI{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "cfapi-system",
			Name:      cfAPIName,
		},
	})).To(Succeed())

	deleteNamespace("cf")
	Expect(deleteHelmChart("cfapi-system", "btp-service-broker")).To(Succeed())
	Expect(deleteHelmChart("korifi", "korifi")).To(Succeed())
	deleteNamespace("korifi")
	Eventually(func(g Gomega) {
		g.Expect(helpers.DeleteYamlFilesInDir(ctx, "../../../dependencies/gateway-api")).To(Succeed())
	}).Should(Succeed())
	Eventually(func(g Gomega) {
		g.Expect(helpers.DeleteYamlFilesInDir(ctx, "../../../dependencies/kpack")).To(Succeed())
	}).Should(Succeed())
	Eventually(func(g Gomega) {
		g.Expect(helpers.DeleteYamlFilesInDir(ctx, "../../../dependencies/metrics-server-local")).To(Succeed())
	}).Should(Succeed())
	Eventually(func(g Gomega) {
		g.Expect(helpers.DeleteYamlFilesInDir(ctx, "../../../module-data/envoy-filter")).To(Succeed())
	}).Should(Succeed())
})

func createK8sClient() client.Client {
	config, err := controllerruntime.GetConfig()
	Expect(err).NotTo(HaveOccurred())

	k, err := client.New(config, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	return k
}

func deleteNamespace(nsName string) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
		},
	}

	Expect(k8sClient.Delete(ctx, ns)).To(Succeed())
	Eventually(func(g Gomega) {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(ns), ns)
		g.Expect(k8serrors.IsNotFound(err)).To(BeTrue())
	}).Should(Succeed())
}

func deleteHelmChart(ns string, chart string) error {
	settings := cli.New()
	actionConfig := new(action.Configuration)
	err := actionConfig.Init(settings.RESTClientGetter(), ns,
		"secret", golog.Printf)
	if err != nil {
		return err
	}

	uninstallClient := action.NewUninstall(actionConfig)
	uninstallClient.IgnoreNotFound = true
	uninstallClient.Wait = true
	uninstallClient.Timeout = 5 * time.Minute

	_, err = uninstallClient.Run(chart)
	return err
}
