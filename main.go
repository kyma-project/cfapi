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
	"flag"
	"os"
	"time"

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

	v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/cfapi"
	"github.com/kyma-project/cfapi/controllers/cfapi/secrets"
	"github.com/kyma-project/cfapi/controllers/helm"
	"github.com/kyma-project/cfapi/controllers/installable"
	"github.com/kyma-project/cfapi/controllers/installable/values"
	"github.com/kyma-project/cfapi/controllers/kyma"
	kymaistiov1alpha2 "github.com/kyma-project/istio/operator/api/v1alpha2"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
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

type FlagVar struct {
	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string
	// TODO: Remove this one, it sets the "final" state on the controller, which sounds wrong on many levels
	finalState string
	// TODO: Remove this one, it setting a state on a controller sounds wrong on many levels
	finalDeletionState string
}

func init() { //nolint:gochecknoinits
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(kymaistiov1alpha2.AddToScheme(scheme))
	utilruntime.Must(apiextv1.AddToScheme(scheme))
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
	installables := []installable.Installable{
		installable.NewYaml(mgr.GetClient(), "./module-data/namespaces/namespaces.yaml", "Namespaces"),
		installable.NewYaml(mgr.GetClient(), "./module-data/vendor/gateway-api/experimental-install.yaml", "Gateway API"),
		installable.NewYaml(mgr.GetClient(), "./module-data/vendor/kpack/release-*.yaml", "kpack"),
		installable.NewHelmChart("./module-data/korifi-prerequisites-chart", "korifi", "korifi-prerequisites", values.NewPrerequisites(), helmClient),
		installable.NewHelmChart("./module-data/vendor/korifi-chart", "korifi", "korifi", values.NewKorifi(), helmClient),
		installable.NewHelmChart("./module-data/cfapi-config-chart", "korifi", "cfapi-config", values.NewCFAPIConfig(), helmClient),
		installable.NewHelmChart("./module-data/btp-service-broker/helm", "cfapi-system", "btp-service-broker", values.NoValues{}, helmClient),
	}

	controllersLog := ctrl.Log.WithName(operatorName)
	if err := cfapi.NewReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		kyma.NewClient(mgr.GetClient(), istioclient.NewForConfigOrDie(ctrl.GetConfigOrDie())),
		secrets.NewDocker(mgr.GetClient()),
		mgr.GetEventRecorderFor(operatorName),
		controllersLog,
		10*time.Second,
		installables...,
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
	flag.StringVar(&flagVar.finalState, "final-state", string(v1alpha1.StateReady),
		"Customize final state, to mimic state behaviour like Ready, Warning")
	flag.StringVar(&flagVar.finalDeletionState, "final-deletion-state", string(v1alpha1.StateDeleting),
		"Customize final state when module marked for deletion, to mimic state behaviour like Ready, Warning")
	return flagVar
}
