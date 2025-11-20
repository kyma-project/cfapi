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

package main

import (
	"context"
	"flag"
	"os"
	"time"

	korifiv1alpha1 "code.cloudfoundry.org/korifi/controllers/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	certv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/cfapi"
	"github.com/kyma-project/cfapi/controllers/cfapi/secrets"
	"github.com/kyma-project/cfapi/controllers/helm"
	"github.com/kyma-project/cfapi/controllers/installable"
	"github.com/kyma-project/cfapi/controllers/installable/values"
	"github.com/kyma-project/cfapi/controllers/kyma"
	kymaistiov1alpha2 "github.com/kyma-project/istio/operator/api/v1alpha2"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	//+kubebuilder:scaffold:imports
)

const (
	operatorName = "cfapi-operator"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

var ContourEnabled installable.Predicate = func(ctx context.Context, config v1alpha1.InstallationConfig) bool {
	return config.GatewayType == v1alpha1.GatewayTypeContour
}

type FlagVar struct {
	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string
}

func init() { //nolint:gochecknoinits
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(kymaistiov1alpha2.AddToScheme(scheme))
	utilruntime.Must(apiextv1.AddToScheme(scheme))
	utilruntime.Must(certv1alpha1.AddToScheme(scheme))
	utilruntime.Must(istiov1beta1.AddToScheme(scheme))
	utilruntime.Must(korifiv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

//nolint:gochecknoglobals
var buildVersion = "not_provided"

func main() {
	flagVar := defineFlagVar()
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	setupLog.Info("Starting CFAPI Operator", "version", buildVersion)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: flagVar.metricsAddr,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 9443,
		}),
		HealthProbeBindAddress: flagVar.probeAddr,
		LeaderElection:         flagVar.enableLeaderElection,
		LeaderElectionID:       "76223278.kyma-project.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	helmClient := helm.NewClient()
	systemNs := installable.NewYaml(mgr.GetClient(), "./module-data/namespaces/system.yaml", "System Namespaces")
	cfRootNs := installable.NewYaml(mgr.GetClient(), "./module-data/namespaces/cfroot.yaml", "Root Namespace")
	certIssuers := installable.NewYaml(mgr.GetClient(), "./module-data/issuers/issuers.yaml", "CertIssuers")
	gwAPI := installable.NewYaml(mgr.GetClient(), "./module-data/vendor/gateway-api/experimental-install.yaml", "Gateway API")
	contour := installable.NewConditional(
		ContourEnabled,
		installable.NewHelmChart("./module-data/vendor/contour-chart", "cfapi-system", "contour", values.ContourValues, helmClient),
	)
	kpack := installable.NewYaml(mgr.GetClient(), "./module-data/vendor/kpack/release-*.yaml", "kpack")
	korifiPrerequisites := installable.NewHelmChart("./module-data/korifi-prerequisites-chart", "korifi", "korifi-prerequisites", values.NewPrerequisites(mgr.GetClient()), helmClient)
	korifi := installable.NewHelmChart("./module-data/vendor/korifi-chart", "korifi", "korifi", values.NewKorifi(mgr.GetClient(), "korifi"), helmClient)
	cfAPIConfig := installable.NewHelmChart("./module-data/cfapi-config-chart", "korifi", "cfapi-config", values.NewCFAPIConfig(mgr.GetClient()), helmClient)
	btpServiceBroker := installable.NewHelmChart("./module-data/btp-service-broker/helm", "cfapi-system", "btp-service-broker", values.Override{}, helmClient)

	installOrder := []installable.Installable{
		systemNs,
		cfRootNs,
		certIssuers,
		gwAPI,
		contour,
		kpack,
		korifiPrerequisites,
		korifi,
		cfAPIConfig,
		btpServiceBroker,
	}

	uninstallOrder := []installable.Installable{
		installable.NewOrgs(mgr.GetClient()),
		cfRootNs,
		btpServiceBroker,
		cfAPIConfig,
		korifi,
		korifiPrerequisites,
		kpack,
		contour,
		gwAPI,
		certIssuers,
		systemNs,
	}

	controllersLog := ctrl.Log.WithName(operatorName)
	if err := cfapi.NewReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		kyma.NewClient(mgr.GetClient()),
		secrets.NewDocker(mgr.GetClient()),
		mgr.GetEventRecorderFor(operatorName),
		controllersLog,
		10*time.Second,
		installOrder,
		uninstallOrder,
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CFAPI")
		os.Exit(1)
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func defineFlagVar() *FlagVar {
	flagVar := new(FlagVar)
	flag.StringVar(&flagVar.metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&flagVar.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&flagVar.enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	return flagVar
}
