// Package btp_test
package btp_test

import (
	"context"
	"log"
	"path/filepath"
	"testing"

	btpv1 "github.com/SAP/sap-btp-service-operator/api/v1"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestBtp(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Btp Suite")
}

var (
	ctx               context.Context
	testEnv           *envtest.Environment
	k8sClient         client.WithWatch
	resourceNamespace string
)

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "tests", "vendor", "sap-btp-service-operator"),
		},
		ErrorIfCRDPathMissing: true,
	}

	_, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())

	Expect(btpv1.AddToScheme(scheme.Scheme)).To(Succeed())

	k8sClient, err = client.NewWithWatch(testEnv.Config, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	Expect(testEnv.Stop()).To(Succeed())
})

var _ = BeforeEach(func() {
	ctx = context.Background()

	ctx = logr.NewContext(ctx, stdr.New(log.New(GinkgoWriter, ">>>", log.LstdFlags)))

	resourceNamespace = uuid.NewString()

	Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: resourceNamespace}})).To(Succeed())
})
