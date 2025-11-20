package helpers

import (
	"context"

	. "github.com/onsi/ginkgo/v2" //lint:ignore ST1001 this is a test file
	. "github.com/onsi/gomega"    //lint:ignore ST1001 this is a test file

	"github.com/kyma-project/cfapi/tools/k8s"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func EnsureCreate(k8sClient client.Client, obj client.Object) {
	GinkgoHelper()

	Expect(k8sClient.Create(context.Background(), obj)).To(Succeed())
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(obj), obj)).To(Succeed())
	}).Should(Succeed())
}

func EnsurePatch[T client.Object](k8sClient client.Client, obj T, modifyFunc func(T)) {
	GinkgoHelper()

	Expect(k8s.Patch(context.Background(), k8sClient, obj, func() {
		modifyFunc(obj)
	})).To(Succeed())
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(obj), obj)).To(Succeed())
		objCopy := k8s.DeepCopy(obj)
		modifyFunc(objCopy)
		g.Expect(equality.Semantic.DeepEqual(objCopy, obj)).To(BeTrue())
	}).Should(Succeed())
}

func EnsureDelete(k8sClient client.Client, obj client.Object) {
	GinkgoHelper()

	Expect(k8sClient.Delete(context.Background(), obj)).To(Succeed())
	Eventually(func(g Gomega) {
		err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(obj), obj)
		g.Expect(k8serrors.IsNotFound(err)).To(BeTrue())
	}).Should(Succeed())
}
