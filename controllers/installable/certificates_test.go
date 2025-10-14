package installable_test

import (
	"errors"

	v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/installable"
	"github.com/kyma-project/cfapi/controllers/installable/fake"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Certificates", func() {
	var (
		installConfig   v1alpha1.InstallationConfig
		yamlInstallable *fake.Installable
		certificates    *installable.Certificates
		result          installable.Result
		err             error
	)

	BeforeEach(func() {
		yamlInstallable = new(fake.Installable)
		yamlInstallable.InstallReturns(installable.Result{
			State:   installable.ResultStateSuccess,
			Message: "success",
		}, nil)
		installConfig = v1alpha1.InstallationConfig{
			RootNamespace: "my-root-ns",
		}

		certificates = installable.NewCertificates(adminClient, yamlInstallable)
	})

	JustBeforeEach(func() {
		result, err = certificates.Install(ctx, installConfig, eventRecorder)
	})

	It("installs the yamls", func() {
		Expect(err).NotTo(HaveOccurred())
		Expect(yamlInstallable.InstallCallCount()).To(Equal(1))
		_, actualConfig, _ := yamlInstallable.InstallArgsForCall(0)
		Expect(actualConfig).To(Equal(installConfig))
	})

	It("returns in progress result", func() {
		Expect(err).NotTo(HaveOccurred())
		Expect(result.State).To(Equal(installable.ResultStateInProgress))
		Expect(result.Message).To(ContainSubstring("Certificates being installed"))
	})

	When("the yaml installable does not return success result", func() {
		BeforeEach(func() {
			yamlInstallable.InstallReturns(installable.Result{
				State:   installable.ResultStateFailed,
				Message: "failure",
			}, nil)
		})

		It("returns that result", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(installable.Result{
				State:   installable.ResultStateFailed,
				Message: "failure",
			}))
		})
	})

	When("the yaml installable returns an error", func() {
		BeforeEach(func() {
			yamlInstallable.InstallReturns(installable.Result{}, errors.New("some error"))
		})

		It("returns the error", func() {
			Expect(err).To(MatchError(ContainSubstring("some error")))
		})
	})

	When("the certificates secrets exist", func() {
		BeforeEach(func() {
			certSecrets := []string{
				"korifi-api-ingress-cert",
				"korifi-api-internal-cert",
				"korifi-workloads-ingress-cert",
				"korifi-controllers-webhook-cert",
				"korifi-kpack-image-builder-webhook-cert",
				"korifi-statefulset-runner-webhook-cert",
			}

			Expect(adminClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "korifi",
				},
			})).To(Succeed())
			for _, secretName := range certSecrets {
				Expect(adminClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "korifi",
						Name:      secretName,
					},
				})).To(Succeed())
			}
		})

		It("returns success result", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.State).To(Equal(installable.ResultStateSuccess))
			Expect(result.Message).To(ContainSubstring("Certificates installed successfully"))
		})
	})
})
