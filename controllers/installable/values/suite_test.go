package values_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kyma-project/cfapi/tests/helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	certv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

var (
	stopClientCache context.CancelFunc
	testEnv         *envtest.Environment
	adminClient     client.Client
	ctx             context.Context
	testNamepace    string
)

func TestNetworkingControllers(t *testing.T) {
	SetDefaultEventuallyTimeout(10 * time.Second)
	SetDefaultEventuallyPollingInterval(250 * time.Millisecond)

	SetDefaultConsistentlyDuration(5 * time.Second)
	SetDefaultConsistentlyPollingInterval(250 * time.Millisecond)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Values Suite")
}

var _ = BeforeEach(func() {
	ctx = context.Background()

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	testEnv = &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths: []string{
			"../../../tests/dependencies/vendor/gardener-cert-manager",
		},
	}

	_, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())

	Expect(certv1alpha1.AddToScheme(testEnv.Scheme)).To(Succeed())

	adminClient, stopClientCache = helpers.NewCachedClient(testEnv.Config)

	Expect(adminClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kyma-system",
		},
	})).To(Succeed())
	Expect(adminClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cfapi-system",
		},
	})).To(Succeed())

	testNamepace = uuid.NewString()
	Expect(adminClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamepace,
		},
	})).To(Succeed())
})

var _ = AfterEach(func() {
	stopClientCache()
	Expect(testEnv.Stop()).To(Succeed())
})
