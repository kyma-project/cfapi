package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"code.cloudfoundry.org/brokerapi/v13"
	btpv1 "github.com/SAP/sap-btp-service-operator/api/v1"
	"github.com/SAP/sap-btp-service-operator/client/sm"
	"github.com/kyma-project/cfapi/components/btp-service-broker/btp"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	utilruntime.Must(btpv1.AddToScheme(scheme.Scheme))
}

type config struct {
	Port              int    `yaml:"port"`
	ResourceNamespace string `yaml:"resource_namespace"`
}

func loadConfig() (config, error) {
	cfgBytes, err := os.ReadFile("/etc/config/broker.yaml")
	if err != nil {
		return config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg config
	if err := yaml.Unmarshal(cfgBytes, &cfg); err != nil {
		return config{}, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

func main() {
	serverConfig, err := loadConfig()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	k8sClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{})
	if err != nil {
		slog.Error("Failed to create kubernetes client", "error", err)
		os.Exit(1)
	}

	smClientConfig, err := loadServiceManagerClientConfig(context.Background(), k8sClient)
	if err != nil {
		slog.Error("Failed to load service manager client configuration", "error", err)
		os.Exit(1)
	}

	smClient, err := sm.NewClient(context.Background(), smClientConfig, nil)
	if err != nil {
		slog.Error("Failed to create service manager client", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr: fmt.Sprintf(":%d", serverConfig.Port),
		Handler: brokerapi.New(
			btp.NewBroker(k8sClient, smClient, serverConfig.ResourceNamespace),
			slog.Default(),
			brokerapi.BrokerCredentials{
				Username: os.Getenv("BROKER_USERNAME"),
				Password: os.Getenv("BROKER_PASSWORD"),
			},
			brokerapi.WithAdditionalMiddleware(loggingMiddleware),
		),
	}

	if err := server.ListenAndServe(); err != nil {
		slog.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Received request", "method", r.Method, "url", r.URL.String())
		next.ServeHTTP(w, r)
		slog.Info("Response sent", "status", w.Header().Get("Status"))
	})
}

func loadServiceManagerClientConfig(ctx context.Context, k8sClient client.Client) (*sm.ClientConfig, error) {
	btpOperatorSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "kyma-system",
			Name:      "sap-btp-manager",
		},
	}

	err := k8sClient.Get(ctx, client.ObjectKeyFromObject(btpOperatorSecret), btpOperatorSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to get sap-btp-manager secret: %w", err)
	}

	return &sm.ClientConfig{
		ClientID:     string(btpOperatorSecret.Data["clientid"]),
		ClientSecret: string(btpOperatorSecret.Data["clientsecret"]),
		URL:          string(btpOperatorSecret.Data["sm_url"]),
		TokenURL:     string(btpOperatorSecret.Data["tokenurl"]) + "/oauth/token",
	}, nil
}
