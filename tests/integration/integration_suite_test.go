package integration_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

const yamlDelimiter = "---\napiVersion"

func init() {
	utilruntime.Must(corev1.AddToScheme(scheme.Scheme))
}

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var (
	ctx       context.Context
	k8sClient client.Client
)

func commonTestSetup() {
	SetDefaultEventuallyTimeout(4 * time.Minute)
	SetDefaultEventuallyPollingInterval(2 * time.Second)
}

var _ = SynchronizedBeforeSuite(func() []byte {
	commonTestSetup()

	ctx = context.Background()
	Eventually(func(g Gomega) {
		g.Expect(applyYamlFile(ctx, filepath.Join(mustGetEnv("CFAPI_MODULE_RELEASE_DIR"), "cfapi-operator.yaml"))).To(Succeed())
		g.Expect(applyYamlFile(ctx, filepath.Join(mustGetEnv("CFAPI_MODULE_RELEASE_DIR"), "cfapi-default-cr.yaml"))).To(Succeed())
	}).Should(Succeed())

	return nil
}, func(bs []byte) {
	commonTestSetup()

	ctx = context.Background()
	k8sClient = createK8sClient()
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	Eventually(func(g Gomega) {
		g.Expect(deleteYamlFile(ctx, filepath.Join(mustGetEnv("CFAPI_MODULE_RELEASE_DIR"), "cfapi-default-cr.yaml"))).To(Succeed())
		g.Expect(deleteYamlFile(ctx, filepath.Join(mustGetEnv("CFAPI_MODULE_RELEASE_DIR"), "cfapi-operator.yaml"))).To(Succeed())
	}).Should(Succeed())
})

func mustGetEnv(envVar string) string {
	envVarValue, ok := os.LookupEnv(envVar)
	Expect(ok).To(BeTrue())

	return envVarValue
}

func createK8sClient() client.Client {
	config, err := controllerruntime.GetConfig()
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err := client.New(config, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	return k8sClient
}

func createDynamicClient() dynamic.Interface {
	config, err := controllerruntime.GetConfig()
	Expect(err).NotTo(HaveOccurred())

	dynClient, err := dynamic.NewForConfig(config)
	Expect(err).NotTo(HaveOccurred())

	return dynClient
}

func createDynamicRestMapper() meta.RESTMapper {
	config, err := controllerruntime.GetConfig()
	Expect(err).NotTo(HaveOccurred())

	httpClient, err := rest.HTTPClientFor(config)
	Expect(err).NotTo(HaveOccurred())

	mapper, err := apiutil.NewDynamicRESTMapper(config, httpClient)
	Expect(err).NotTo(HaveOccurred())

	return mapper
}

func applyYamlFile(ctx context.Context, yamlFilePath string) error {
	yamlDocs, err := parseYamlIntoDocs(yamlFilePath)
	if err != nil {
		return err
	}

	for _, doc := range yamlDocs {
		resourceClient, obj, err := resourceClientFor(doc)
		if err != nil {
			return err
		}
		_, err = resourceClient.Create(ctx, obj, metav1.CreateOptions{})
		if client.IgnoreAlreadyExists(err) != nil {
			return err
		}
	}

	return nil
}

func deleteYamlFile(ctx context.Context, yamlFilePath string) error {
	yamlDocs, err := parseYamlIntoDocs(yamlFilePath)
	if err != nil {
		return err
	}

	for _, doc := range yamlDocs {
		resourceClient, obj, err := resourceClientFor(doc)
		if err != nil {
			return err
		}

		deleteForeground := metav1.DeletePropagationForeground
		err = resourceClient.Delete(ctx, obj.GetName(), metav1.DeleteOptions{
			PropagationPolicy: &deleteForeground,
		})
		if err != nil {
			return err
		}

		_, err = resourceClient.Get(ctx, obj.GetName(), metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func parseYamlIntoDocs(yamlFilePath string) ([][]byte, error) {
	yamlData, err := os.ReadFile(yamlFilePath)
	if err != nil {
		return nil, err
	}

	yamlDocs := [][]byte{}
	splitDocs := bytes.Split(yamlData, []byte(yamlDelimiter))
	for i, doc := range splitDocs {
		// Only restore the stripped prefix for the second and subsequent elements (the first element does not start with the separator)
		if i != 0 {
			doc = append([]byte(yamlDelimiter), doc...)
		}

		yamlDocs = append(yamlDocs, doc)
	}

	return yamlDocs, nil
}

func resourceClientFor(yamlDoc []byte) (dynamic.ResourceInterface, *unstructured.Unstructured, error) {
	dynClient := createDynamicClient()
	decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	obj := &unstructured.Unstructured{}
	_, gvk, err := decoder.Decode(yamlDoc, nil, obj)
	if err != nil {
		return nil, nil, err
	}

	mapper := createDynamicRestMapper()
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, nil, err
	}

	return dynClient.Resource(mapping.Resource).Namespace(obj.GetNamespace()), obj, nil
}
