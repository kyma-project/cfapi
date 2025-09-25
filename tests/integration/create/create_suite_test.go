package create_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyma-project/cfapi/tests/integration/helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/kubectl/pkg/scheme"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Create Integration Suite")
}

var (
	ctx       context.Context
	k8sClient client.Client
	fixture   *helpers.Fixture
)

func commonTestSetup() {
	SetDefaultEventuallyTimeout(4 * time.Minute)
	SetDefaultEventuallyPollingInterval(2 * time.Second)

	k8sClient = createK8sClient()
}

var _ = SynchronizedBeforeSuite(func() []byte {
	commonTestSetup()

	ctx = context.Background()
	fixture = helpers.NewFixture(k8sClient)

	fixture.SetUp(ctx)
	Eventually(func(g Gomega) {
		g.Expect(helpers.ApplyYamlFile(ctx, filepath.Join(helpers.MustGetEnv("CFAPI_MODULE_RELEASE_DIR"), "cfapi-default-cr.yaml"))).To(Succeed())
	}).Should(Succeed())

	return nil
}, func(bs []byte) {
	commonTestSetup()

	ctx = context.Background()
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	Eventually(func(g Gomega) {
		g.Expect(helpers.DeleteYamlFile(ctx, filepath.Join(helpers.MustGetEnv("CFAPI_MODULE_RELEASE_DIR"), "cfapi-default-cr.yaml"))).To(Succeed())
	}).Should(Succeed())

	fixture.TearDown(ctx)
})

func createK8sClient() client.Client {
	config, err := controllerruntime.GetConfig()
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err := client.New(config, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	return k8sClient
}
