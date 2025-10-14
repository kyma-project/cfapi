package helm_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/scheme"

	corev1 "k8s.io/api/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Helm Integration Suite")
}

var (
	ctx           context.Context
	k8sClient     client.Client
	testNamespace string
)

var _ = BeforeSuite(func() {
	SetDefaultEventuallyTimeout(1 * time.Minute)
	SetDefaultEventuallyPollingInterval(2 * time.Second)
})

var _ = BeforeEach(func() {
	ctx = context.Background()
	k8sClient = createK8sClient()

	testNamespace = uuid.NewString()
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
		},
	}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed())
	DeferCleanup(func() {
		Expect(k8sClient.Delete(ctx, ns)).To(Succeed())
	})
})

func createK8sClient() client.Client {
	config, err := controllerruntime.GetConfig()
	Expect(err).NotTo(HaveOccurred())

	k, err := client.New(config, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	return k
}
