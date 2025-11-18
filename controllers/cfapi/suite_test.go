package cfapi_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/cfapi"
	"github.com/kyma-project/cfapi/controllers/cfapi/secrets"
	"github.com/kyma-project/cfapi/controllers/installable"
	"github.com/kyma-project/cfapi/controllers/installable/fake"
	"github.com/kyma-project/cfapi/controllers/kyma"
	"github.com/kyma-project/cfapi/tests/helpers"
	"github.com/kyma-project/cfapi/tools"
	kymaistiov1alpha2 "github.com/kyma-project/istio/operator/api/v1alpha2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istiov1alpha3 "istio.io/api/networking/v1alpha3"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	//+kubebuilder:scaffold:imports
)

var (
	stopManager     context.CancelFunc
	stopClientCache context.CancelFunc
	testEnv         *envtest.Environment
	k8sManager      manager.Manager
	adminClient     client.Client
	ctx             context.Context
	cfAPINamespace  string

	firstToInstall  *fake.Installable
	secondToInstall *fake.Installable

	firstToUninstall  *fake.Installable
	secondToUninstall *fake.Installable
)

func TestNetworkingControllers(t *testing.T) {
	SetDefaultEventuallyTimeout(10 * time.Second)
	SetDefaultEventuallyPollingInterval(250 * time.Millisecond)

	SetDefaultConsistentlyDuration(5 * time.Second)
	SetDefaultConsistentlyPollingInterval(250 * time.Millisecond)

	RegisterFailHandler(Fail)
	RunSpecs(t, "CFAPI Controller Suite")
}

var _ = BeforeEach(func() {
	ctx = context.Background()

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
			filepath.Join("..", "..", "tests", "dependencies", "vendor", "kyma-docker-registry"),
			filepath.Join("..", "..", "tests", "dependencies", "vendor", "istio-kyma"),
			filepath.Join("..", "..", "tests", "dependencies", "vendor", "istio", "manifests", "charts", "base", "files"),
		},
		ErrorIfCRDPathMissing: true,
	}

	_, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())

	Expect(v1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(istiov1beta1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(kymaistiov1alpha2.AddToScheme(testEnv.Scheme)).To(Succeed())

	k8sManager = helpers.NewK8sManager(testEnv, filepath.Join("config", "rbac", "role.yaml"))

	adminClient, stopClientCache = helpers.NewCachedClient(testEnv.Config)

	cfAPINamespace = uuid.NewString()
	Expect(adminClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: cfAPINamespace,
		},
	})).To(Succeed())
	Expect(adminClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kyma-system",
		},
	})).To(Succeed())

	Expect(adminClient.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cfAPINamespace,
			Name:      kyma.ContainerRegistrySecretName,
		},
		StringData: map[string]string{
			"pushRegAddr": "https://kyma-registry.com",
		},
	})).To(Succeed())

	Expect(adminClient.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "kyma-system",
			Name:      "sap-btp-service-operator",
		},
		StringData: map[string]string{
			"tokenurl": "https://worker1-q3zjpctt.authentication.eu12.hana.ondemand.com",
		},
	})).To(Succeed())

	Expect(adminClient.Create(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default-kyma-admins",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "User",
			Name:      "default.admin@sap.com",
			Namespace: "kyma-system",
		}},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: "cluster-admin",
		},
	})).To(Succeed())

	istioClient := istioclient.NewForConfigOrDie(testEnv.Config)
	_, err = istioClient.NetworkingV1beta1().Gateways("kyma-system").Create(
		ctx,
		&istiov1beta1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "kyma-system",
				Name:      "kyma-gateway",
			},
			Spec: istiov1alpha3.Gateway{
				Servers: []*istiov1alpha3.Server{{
					Hosts: []string{"*.kyma-host.com"},
					Port: tools.PtrTo(istiov1alpha3.Port{
						Name:     "whatever",
						Number:   30111,
						Protocol: "https",
					}),
				}},
			},
		},
		metav1.CreateOptions{},
	)

	Expect(err).NotTo(HaveOccurred())

	istio := &kymaistiov1alpha2.Istio{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "kyma-system",
			Name:      "default",
		},
		Spec: kymaistiov1alpha2.IstioSpec{
			Experimental: &kymaistiov1alpha2.Experimental{
				PilotFeatures: kymaistiov1alpha2.PilotFeatures{
					EnableAlphaGatewayAPI: true,
				},
			},
		},
	}
	Expect(adminClient.Create(ctx, istio)).To(Succeed())

	firstToInstall = new(fake.Installable)
	secondToInstall = new(fake.Installable)
	firstToUninstall = new(fake.Installable)
	secondToUninstall = new(fake.Installable)

	kymaClient := kyma.NewClient(adminClient)
	err = cfapi.NewReconciler(
		k8sManager.GetClient(),
		k8sManager.GetScheme(),
		kymaClient,
		secrets.NewDocker(adminClient),
		k8sManager.GetEventRecorderFor("cfapi"),
		ctrl.Log.WithName("controllers").WithName("cfapi"),
		100*time.Millisecond,
		[]installable.Installable{firstToInstall, secondToInstall},
		[]installable.Installable{firstToUninstall, secondToUninstall},
	).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	stopManager = helpers.StartK8sManager(k8sManager)
})

var _ = AfterEach(func() {
	stopManager()
	stopClientCache()
	Expect(testEnv.Stop()).To(Succeed())
})
