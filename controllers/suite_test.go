/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
/*
 * SPDX-FileCopyrightText: 2024 Samir Zeort <samir.zeort@sap.com>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package controllers_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"istio.io/api/networking/v1alpha3"
	istiogw "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"istio.io/istio/pkg/kube"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorkymaprojectiov1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
	controllers "github.com/kyma-project/cfapi/controllers"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	k8sClient  client.Client                //nolint:gochecknoglobals
	k8sManager manager.Manager              //nolint:gochecknoglobals
	testEnv    *envtest.Environment         //nolint:gochecknoglobals
	ctx        context.Context              //nolint:gochecknoglobals
	cancel     context.CancelFunc           //nolint:gochecknoglobals
	reconciler *controllers.CFAPIReconciler //nolint:gochecknoglobals
)

const (
	testChartPath               = "./test/busybox"
	rateLimiterBurstDefault     = 200
	rateLimiterFrequencyDefault = 30
	failureBaseDelayDefault     = 1 * time.Second
	failureMaxDelayDefault      = 1000 * time.Second
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.Background())
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "config", "crd", "bases"),
			filepath.Join("..", "tests", "vendor", "istio", "crd-all.gen.yaml"),
			filepath.Join("..", "tests", "vendor", "docker-registry", "dockerregistry-operator.yaml"),
		},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = controllers.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = controllers.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err = ctrl.NewManager(
		cfg, ctrl.Options{
			Scheme: scheme.Scheme,
		})
	Expect(err).ToNot(HaveOccurred())

	reconciler = &controllers.CFAPIReconciler{
		Client:             k8sManager.GetClient(),
		Scheme:             scheme.Scheme,
		EventRecorder:      k8sManager.GetEventRecorderFor("tests"),
		FinalState:         operatorkymaprojectiov1alpha1.StateReady,
		FinalDeletionState: operatorkymaprojectiov1alpha1.StateDeleting,
		ModuleDataPath:     "../module-data",
	}

	err = reconciler.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kyma-system",
		},
	})).To(Succeed())
	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cfapi-system",
		},
	})).To(Succeed())
	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "istio-system",
		},
	})).To(Succeed())

	createKymaGateway(ctx, cfg)

	Expect(k8sClient.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "kyma-system",
			Name:      "sap-btp-service-operator",
		},
		StringData: map[string]string{
			"tokenurl": "https://my-token.example.org",
		},
	})).To(Succeed())

	Expect(k8sClient.Create(ctx, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "istio-system",
			Name:      "istiod",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"istio": "pilot"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"istio": "pilot"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "discovery",
						Image: "europe-docker.pkg.dev/kyma-project/prod/external/istio/pilot:1.27.1-distroless",
					}},
				},
			},
		},
	})).To(Succeed())
})

var _ = AfterSuite(func() {
	By("canceling the context for the manager to shutdown")
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func createKymaGateway(ctx context.Context, config *rest.Config) {
	istioClient, err := kube.NewClient(kube.NewClientConfigForRestConfig(config), "")
	Expect(err).NotTo(HaveOccurred())

	_, err = istioClient.Istio().NetworkingV1beta1().Gateways("kyma-system").Create(ctx, &istiogw.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "kyma-system",
			Name:      "kyma-gateway",
		},
		Spec: v1alpha3.Gateway{
			Servers: []*v1alpha3.Server{{
				Hosts: []string{
					"*.kind-127-0-0-1.nip.io",
				},
				Port: &v1alpha3.Port{
					Number:   8443,
					Protocol: "HTTPS",
					Name:     "https",
				},
			}},
		},
	}, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
}
