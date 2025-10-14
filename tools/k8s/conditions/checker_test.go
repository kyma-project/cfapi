package conditions_test

import (
	v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/tools/k8s/conditions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("CheckConditionIsTrue", func() {
	var (
		obj      *v1alpha1.CFAPI
		checkErr error
	)
	BeforeEach(func() {
		obj = &v1alpha1.CFAPI{}
	})

	JustBeforeEach(func() {
		checkErr = conditions.CheckConditionIsTrue(obj, "TestCondition")
	})

	It("errors", func() {
		Expect(checkErr).To(MatchError("condition TestCondition not set yet"))
	})

	When("the condition is false", func() {
		BeforeEach(func() {
			obj.Status.Conditions = []metav1.Condition{{
				Type:   "TestCondition",
				Status: metav1.ConditionFalse,
			}}
		})

		It("errors", func() {
			Expect(checkErr).To(MatchError("TestCondition condition is not true"))
		})
	})

	When("the condition is true", func() {
		BeforeEach(func() {
			obj.Status.Conditions = []metav1.Condition{{
				Type:   "TestCondition",
				Status: metav1.ConditionTrue,
			}}
		})

		It("succeeds", func() {
			Expect(checkErr).NotTo(HaveOccurred())
		})
	})

	When("the condition is outdated", func() {
		BeforeEach(func() {
			obj.SetGeneration(2)

			obj.Status.Conditions = []metav1.Condition{{
				Type:               "TestCondition",
				ObservedGeneration: 1,
			}}
		})

		It("errors", func() {
			Expect(checkErr).To(MatchError("condition TestCondition is outdated"))
		})
	})
})
