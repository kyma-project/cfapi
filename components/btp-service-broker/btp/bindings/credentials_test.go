package bindings_test

import (
	"encoding/json"

	"github.com/kyma-project/cfapi/components/btp-service-broker/btp/bindings"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CredentialsDecoder", func() {
	var (
		decoder      *bindings.CredentialsDecoder
		secretData   map[string][]byte
		decodedCreds any
		err          error
	)

	BeforeEach(func() {
		secretData = map[string][]byte{}
		secretData[".metadata"] = mustMarshal(map[string][]map[string]string{
			"credentialProperties": {
				{"name": "plain-value", "format": "text"},
				{"name": "object-value", "format": "json"},
			},
		})
		secretData["plain-value"] = []byte("my-string-value")
		secretData["object-value"] = mustMarshal(map[string]string{"obj-key": "obj-value"})

		decoder = bindings.NewCredentialsDecoder()
	})

	JustBeforeEach(func() {
		decodedCreds, err = decoder.DecodeBindingSecretData(secretData)
	})

	It("returns credentials", func() {
		Expect(err).NotTo(HaveOccurred())
		Expect(decodedCreds).To(Equal(map[string]any{
			"plain-value": "my-string-value",
			"object-value": map[string]any{
				"obj-key": "obj-value",
			},
		}))
	})

	When("secret data is missing .metadata key", func() {
		BeforeEach(func() {
			delete(secretData, ".metadata")
		})

		It("returns missing .metadata key error", func() {
			Expect(err).To(MatchError("secret data does not contain .metadata key"))
		})
	})

	When(".metadata is not valid JSON", func() {
		BeforeEach(func() {
			secretData[".metadata"] = []byte("invalid-json")
		})

		It("returns unmarshal error", func() {
			Expect(err).To(MatchError(ContainSubstring("failed to unmarshal")))
		})
	})

	When("a credential property has invalid format", func() {
		BeforeEach(func() {
			secretData[".metadata"] = mustMarshal(map[string][]map[string]string{
				"credentialProperties": {
					{"name": "plain-value", "format": "invalid-format"},
				},
			})
		})

		It("returns unsupported format error", func() {
			Expect(err).To(MatchError(`unsupported credentials property format "invalid-format" for property "plain-value"`))
		})
	})

	When("secret data does not contain a credential value", func() {
		BeforeEach(func() {
			delete(secretData, "plain-value")
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring(`missing credentials property with name "plain-value"`)))
		})
	})
})

func mustMarshal(v any) []byte {
	GinkgoHelper()

	marshalled, err := json.Marshal(v)
	Expect(err).NotTo(HaveOccurred())
	return marshalled
}
