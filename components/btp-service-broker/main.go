package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	btpv1 "github.com/SAP/sap-btp-service-operator/api/v1"
	"github.com/kyma-project/cfapi/config"
	"github.com/kyma-project/cfapi/handlers"
	"github.com/kyma-project/cfapi/middleware"
	"github.com/kyma-project/cfapi/routing"
	"github.com/kyma-project/cfapi/service_manager"
	"github.com/kyma-project/cfapi/tools"
	
	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"go.uber.org/zap/zapcore"
)

func init() {
	utilruntime.Must(btpv1.AddToScheme(scheme.Scheme))
	utilruntime.Must(corev1.AddToScheme(scheme.Scheme))
}

func main() {
	logger, _, err := tools.NewZapLogger(zapcore.DebugLevel)
	if err != nil {
		panic(fmt.Sprintf("error creating new zap logger: %v", err))
	}
	
	routerBuilder := routing.NewRouterBuilder()
	routerBuilder.UseMiddleware(
		middleware.HTTPLogging,
	)

	serverConfig, err := config.LoadServerConfig()
	if err != nil {
		fmt.Println(err, "failed to load server config")
		os.Exit(1)
	}
	k8sClientConfig := ctrl.GetConfigOrDie()
	k8sClient, err := client.NewWithWatch(k8sClientConfig, client.Options{})
	if err != nil {
		fmt.Println(err, "failed to create k8s client")
		os.Exit(1)
	}
	
	smConfig, err := config.LoadServiceManagerClientConfig()
	if err != nil {
		fmt.Println(err, "failed to create sm client")
		os.Exit(1)
	}
	smClient := service_manager.NewClient(smConfig)

	handlers := []routing.Routable{
		handlers.NewHello(),
		handlers.NewCatalog(smClient),
		handlers.NewServiceIntances(k8sClient, serverConfig.RootNamespace),
		handlers.NewServiceBindings(k8sClient, serverConfig.RootNamespace),
	}

	for _, handler := range handlers {
		routerBuilder.LoadRoutes(handler)
	}

	portString := fmt.Sprintf(":%v", serverConfig.InternalPort)

	srv := &http.Server{
		Addr:              portString,
		Handler:           routerBuilder.Build(),
		IdleTimeout:       time.Duration(serverConfig.IdleTimeout * int(time.Second)),
		ReadTimeout:       time.Duration(serverConfig.ReadTimeout * int(time.Second)),
		ReadHeaderTimeout: time.Duration(serverConfig.ReadHeaderTimeout * int(time.Second)),
		WriteTimeout:      time.Duration(serverConfig.WriteTimeout * int(time.Second)),
		ErrorLog:          log.New(&tools.LogrWriter{Logger: logger, Message: "HTTP server error"}, "", 0),
	}

	logger.Info("listening on " + portString)
	err = srv.ListenAndServe()
	if err != nil {
		logger.Error(err, "error serving HTTP")
		os.Exit(1)
	}
}
