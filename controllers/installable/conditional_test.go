package installable_test

import (
	"context"

	"github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/installable"
	"github.com/kyma-project/cfapi/controllers/installable/fake"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Conditional Installable", func() {
	var (
		predicate installable.Predicate
		config    v1alpha1.InstallationConfig
		delegate  *fake.Installable

		result     installable.Result
		installErr error
	)

	BeforeEach(func() {
		config = v1alpha1.InstallationConfig{
			RootNamespace: "my-root-ns",
		}

		delegate = new(fake.Installable)

		predicate = func(ctx context.Context, config v1alpha1.InstallationConfig) bool {
			return true
		}

		delegate.InstallReturns(installable.Result{
			State:   installable.ResultStateSuccess,
			Message: "install-success",
		}, nil)

		delegate.UninstallReturns(installable.Result{
			State:   installable.ResultStateSuccess,
			Message: "uninstall-success",
		}, nil)
	})

	Describe("Install", func() {
		JustBeforeEach(func() {
			result, installErr = installable.NewConditional(predicate, delegate).Install(ctx, config, eventRecorder)
		})

		It("delegates to the installable", func() {
			Expect(installErr).NotTo(HaveOccurred())
			Expect(delegate.InstallCallCount()).To(Equal(1))
			_, actualConfig, _ := delegate.InstallArgsForCall(0)
			Expect(actualConfig).To(Equal(config))
			Expect(result).To(Equal(installable.Result{
				State:   installable.ResultStateSuccess,
				Message: "install-success",
			}))
		})

		When("the conidtion is not met", func() {
			BeforeEach(func() {
				predicate = func(ctx context.Context, config v1alpha1.InstallationConfig) bool {
					return false
				}
			})

			It("delegates to the installable", func() {
				Expect(installErr).NotTo(HaveOccurred())
				Expect(delegate.UninstallCallCount()).To(Equal(1))
				_, actualConfig, _ := delegate.UninstallArgsForCall(0)
				Expect(actualConfig).To(Equal(config))
				Expect(result).To(Equal(installable.Result{
					State:   installable.ResultStateSuccess,
					Message: "uninstall-success",
				}))
			})
		})
	})

	Describe("Uninstall", func() {
		JustBeforeEach(func() {
			result, installErr = installable.NewConditional(nil, delegate).Uninstall(ctx, config, eventRecorder)
		})

		It("delegates to the installable", func() {
			Expect(installErr).NotTo(HaveOccurred())
			Expect(delegate.UninstallCallCount()).To(Equal(1))
			_, actualConfig, _ := delegate.UninstallArgsForCall(0)
			Expect(actualConfig).To(Equal(config))
			Expect(result).To(Equal(installable.Result{
				State:   installable.ResultStateSuccess,
				Message: "uninstall-success",
			}))
		})
	})
})
