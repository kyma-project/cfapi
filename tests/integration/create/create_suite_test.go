package create_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	golog "log"

	gardenerv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
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

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	buildv1alpha2 "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	v1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/kubectl/pkg/scheme"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme.Scheme))
	utilruntime.Must(controllers.AddToScheme(scheme.Scheme))
	utilruntime.Must(gatewayv1.Install(scheme.Scheme))
	utilruntime.Must(v1alpha3.AddToScheme(scheme.Scheme))
	utilruntime.Must(gardenerv1alpha1.AddToScheme(scheme.Scheme))
	utilruntime.Must(dnsv1alpha1.AddToScheme(scheme.Scheme))
	utilruntime.Must(buildv1alpha2.AddToScheme(scheme.Scheme))
}

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Create Integration Suite")
}

var (
	ctx        context.Context
	k8sClient  client.Client
	cfAPIName  string
	sharedData sharedSetupData
)

type sharedSetupData struct {
	CfAPIName   string `json:"cf_api_name"`
	CfAdminUser string `json:"cf_admin_user"`
}

func commonTestSetup() {
	SetDefaultEventuallyTimeout(4 * time.Minute)
	SetDefaultEventuallyPollingInterval(2 * time.Second)

	ctx = context.Background()
	k8sClient = createK8sClient()
}

var _ = SynchronizedBeforeSuite(func() []byte {
	commonTestSetup()

	cfAdminUser := uuid.NewString()
	Expect(k8sClient.Create(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: cfAdminUser,
		},
		Subjects: []rbacv1.Subject{{
			APIGroup: rbacv1.GroupName,
			Kind:     rbacv1.UserKind,
			Name:     cfAdminUser,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
	})).To(Succeed())

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

	sharedData = sharedSetupData{
		CfAPIName:   cfAPI.Name,
		CfAdminUser: cfAdminUser,
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

	return bs
}, func(bs []byte) {
	commonTestSetup()

	err := json.Unmarshal(bs, &sharedData)
	Expect(err).NotTo(HaveOccurred())

	cfAPIName = sharedData.CfAPIName
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

	Expect(k8sClient.Delete(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: sharedData.CfAdminUser,
		},
	})).To(Succeed())
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
