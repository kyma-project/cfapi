package create_test

import (
	"context"
	"encoding/json"
	"os/exec"
	"testing"
	"time"

	gardenerv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/google/uuid"
	"github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/tests/helpers/fail_handler"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

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
	utilruntime.Must(v1alpha1.AddToScheme(scheme.Scheme))
	utilruntime.Must(gatewayv1.Install(scheme.Scheme))
	utilruntime.Must(v1alpha3.AddToScheme(scheme.Scheme))
	utilruntime.Must(gardenerv1alpha1.AddToScheme(scheme.Scheme))
	utilruntime.Must(dnsv1alpha1.AddToScheme(scheme.Scheme))
	utilruntime.Must(buildv1alpha2.AddToScheme(scheme.Scheme))
}

func TestIntegration(t *testing.T) {
	RegisterFailHandler(fail_handler.New("Create Integration Tests",
		fail_handler.Hook{
			Matcher: fail_handler.Always,
			Hook: func(config *rest.Config, failure fail_handler.TestFailure) {
				fail_handler.PrintCFAPIControllerLogs(config, failure.StartTime)
			},
		},
	).Fail)
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
			UseSelfSignedCertificates: true,
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

	Expect(run("kubectl", "delete", "--ignore-not-found", "namespace", "cf")).To(Succeed())
	Expect(run("helm", "delete", "--ignore-not-found", "btp-service-broker", "-n", "cfapi-system", "--wait")).To(Succeed())
	Expect(run("helm", "delete", "--ignore-not-found", "cfapi-config", "-n", "korifi", "--wait")).To(Succeed())
	Expect(run("helm", "delete", "--ignore-not-found", "korifi", "-n", "korifi", "--wait")).To(Succeed())
	Expect(run("helm", "delete", "--ignore-not-found", "korifi-prerequisites", "-n", "korifi", "--wait")).To(Succeed())
	Expect(run("kubectl", "delete", "--ignore-not-found", "namespace", "korifi")).To(Succeed())
	Expect(run("kubectl", "delete", "--ignore-not-found", "-f", "../../../module-data/issuers/issuers.yaml")).To(Succeed())
	Expect(run("kubectl", "delete", "--ignore-not-found", "-f", "../../../module-data/vendor/gateway-api")).To(Succeed())
	Expect(run("kubectl", "delete", "--ignore-not-found", "-f", "../../../module-data/vendor/kpack")).To(Succeed())

	Expect(k8sClient.Delete(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: sharedData.CfAdminUser,
		},
	})).To(Succeed())
})

func run(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	return cmd.Run()
}

func createK8sClient() client.Client {
	config, err := controllerruntime.GetConfig()
	Expect(err).NotTo(HaveOccurred())

	k, err := client.New(config, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	return k
}
