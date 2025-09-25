package helpers

import (
	"context"
	"path/filepath"

	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Fixture struct {
	k8sClient client.Client
}

func NewFixture(k8sClient client.Client) *Fixture {
	return &Fixture{k8sClient: k8sClient}
}

func (f *Fixture) SetUp(ctx context.Context) {
	Eventually(func(g Gomega) {
		g.Expect(ApplyYamlFile(ctx, filepath.Join(MustGetEnv("CFAPI_MODULE_RELEASE_DIR"), "cfapi-operator.yaml"))).To(Succeed())
	}).Should(Succeed())

	Expect(f.addImagePullSecretToOperatorDeployment(ctx)).To(Succeed())
}

func (f *Fixture) TearDown(ctx context.Context) {
	Eventually(func(g Gomega) {
		g.Expect(DeleteYamlFile(ctx, filepath.Join(MustGetEnv("CFAPI_MODULE_RELEASE_DIR"), "cfapi-operator.yaml"))).To(Succeed())
	}).Should(Succeed())
}

func (f *Fixture) addImagePullSecretToOperatorDeployment(ctx context.Context) error {
	depl := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "cfapi-system",
			Name:      "cfapi-operator",
		},
	}
	if err := f.k8sClient.Get(ctx, client.ObjectKeyFromObject(depl), depl); err != nil {
		return err
	}

	modifiedDepl := depl.DeepCopy()
	modifiedDepl.Spec.Template.Spec.ImagePullSecrets = append(modifiedDepl.Spec.Template.Spec.ImagePullSecrets,
		corev1.LocalObjectReference{Name: "dockerregistry-config"})
	return f.k8sClient.Patch(ctx, modifiedDepl, client.MergeFrom(depl))
}
