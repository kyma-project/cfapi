package secrets_test

import (
	"encoding/json"

	"github.com/kyma-project/cfapi/controllers/cfapi/secrets"
	"github.com/kyma-project/cfapi/tests/helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Docker", func() {
	var (
		config     secrets.DockerRegistryConfig
		err        error
		docker     *secrets.Docker
		secretName string
	)

	BeforeEach(func() {
		secretData := secrets.DockerRegistryConfig{
			Auths: map[string]secrets.DockerRegistryAuth{
				"my-server.com": {
					Username: "my-user",
					Password: "my-password",
				},
			},
		}

		var secretDataBytes []byte
		secretDataBytes, err = json.Marshal(secretData)
		Expect(err).NotTo(HaveOccurred())

		secretName = "docker-secret"
		helpers.EnsureCreate(adminClient, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace,
				Name:      secretName,
			},
			Data: map[string][]byte{
				corev1.DockerConfigJsonKey: secretDataBytes,
			},
		})

		docker = secrets.NewDocker(adminClient)
	})

	JustBeforeEach(func() {
		config, err = docker.GetRegistryConfig(ctx, testNamespace, secretName)
	})

	It("returns the config", func() {
		Expect(err).NotTo(HaveOccurred())
		Expect(config).To(Equal(secrets.DockerRegistryConfig{
			Auths: map[string]secrets.DockerRegistryAuth{
				"my-server.com": {
					Username: "my-user",
					Password: "my-password",
				},
			},
		}))
	})

	When("getting the secret fails", func() {
		BeforeEach(func() {
			secretName = "does-not-exist"
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("failed to get docker registry secret")))
		})
	})

	When("the secret contains invalid data", func() {
		BeforeEach(func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNamespace,
					Name:      secretName,
				},
			}
			Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())

			helpers.EnsurePatch(adminClient, secret, func(s *corev1.Secret) {
				s.Data[corev1.DockerConfigJsonKey] = []byte("invalid-json")
			})
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("failed to unmarshal docker registry config")))
		})
	})
})
