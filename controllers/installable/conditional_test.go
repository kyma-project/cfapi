package installable_test

import (
	"errors"

	"github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/installable"
	"github.com/kyma-project/cfapi/controllers/installable/fake"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Conditional", func() {
	var (
		condition   *fake.Condition
		conditional *installable.Conditional
		config      v1alpha1.InstallationConfig
		delegate    *fake.Installable

		result     installable.Result
		installErr error
	)

	BeforeEach(func() {
		condition = new(fake.Condition)
		condition.IsMetReturns(true, "")
		config = v1alpha1.InstallationConfig{
			RootNamespace: "my-root-ns",
		}

		delegate = new(fake.Installable)
		delegate.InstallReturns(installable.Result{
			State:   installable.ResultStateSuccess,
			Message: "success",
		}, nil)

		conditional = installable.NewConditional(condition, delegate)
	})

	JustBeforeEach(func() {
		result, installErr = conditional.Install(ctx, config, eventRecorder)
	})

	It("checks the condition", func() {
		Expect(installErr).NotTo(HaveOccurred())
		Expect(condition.IsMetCallCount()).To(Equal(1))
		_, actualConfig := condition.IsMetArgsForCall(0)
		Expect(actualConfig).To(Equal(config))
	})

	It("delegates to the installable", func() {
		Expect(installErr).NotTo(HaveOccurred())
		Expect(delegate.InstallCallCount()).To(Equal(1))
		_, actualConfig, _ := delegate.InstallArgsForCall(0)
		Expect(actualConfig).To(Equal(config))
		Expect(result).To(Equal(installable.Result{
			State:   installable.ResultStateSuccess,
			Message: "success",
		}))
	})

	When("the conidtion is not met", func() {
		BeforeEach(func() {
			condition.IsMetReturns(false, "test-condition not met")
		})

		It("does not delegates to the installable", func() {
			Expect(installErr).NotTo(HaveOccurred())
			Expect(delegate.InstallCallCount()).To(BeZero())
		})

		It("returns in progress result", func() {
			Expect(installErr).NotTo(HaveOccurred())
			Expect(result).To(Equal(installable.Result{
				State:   installable.ResultStateInProgress,
				Message: "test-condition not met",
			}))
		})
	})

	When("the delegate fails", func() {
		BeforeEach(func() {
			delegate.InstallReturns(installable.Result{}, errors.New("delegate error"))
		})

		It("returns the error", func() {
			Expect(installErr).To(MatchError(ContainSubstring("delegate error")))
		})
	})
})
