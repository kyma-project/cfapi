package tools_test

import (
	"time"

	"github.com/kyma-project/cfapi/components/btp-service-broker/tools"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

var _ = Describe("Compare", func() {
	DescribeTable("ZeroIfNil",
		func(value *time.Time, match types.GomegaMatcher) {
			Expect(tools.ZeroIfNil(value)).To(match)
		},
		Entry("nil", nil, BeZero()),
		Entry("not nil", tools.PtrTo(time.UnixMilli(1)), Equal(time.UnixMilli(1))),
	)
})
