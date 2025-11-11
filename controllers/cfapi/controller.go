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

package cfapi

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/BooleanCat/go-functional/v2/it"
	"github.com/go-logr/logr"
	v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/cfapi/secrets"
	"github.com/kyma-project/cfapi/controllers/installable"
	"github.com/kyma-project/cfapi/controllers/kyma"
	"github.com/kyma-project/cfapi/tools/k8s"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
)

const (
	Finalizer = "cfapi.kyma-project.io/finalizer"
)

type Reconciler struct {
	k8sClient       client.Client
	scheme          *runtime.Scheme
	kymaClient      *kyma.Client
	docker          *secrets.Docker
	eventRecorder   record.EventRecorder
	requeueInterval time.Duration
	installables    []installable.Installable
}

func NewReconciler(
	k8sClient client.Client,
	scheme *runtime.Scheme,
	kymaClient *kyma.Client,
	docker *secrets.Docker,
	eventRecorder record.EventRecorder,
	log logr.Logger,
	requeueInterval time.Duration,
	installables ...installable.Installable,
) *k8s.PatchingReconciler[v1alpha1.CFAPI] {
	apiReconciler := &Reconciler{
		k8sClient:       k8sClient,
		scheme:          scheme,
		kymaClient:      kymaClient,
		docker:          docker,
		eventRecorder:   eventRecorder,
		requeueInterval: requeueInterval,
		installables:    installables,
	}
	return k8s.NewPatchingReconciler(ctrl.Log, k8sClient, apiReconciler)
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) *builder.Builder {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.CFAPI{})
}

func (r *Reconciler) ReconcileResource(ctx context.Context, cfAPI *v1alpha1.CFAPI) (ctrl.Result, error) {
	log := logr.FromContextOrDiscard(ctx)

	cfAPI.Status.ObservedGeneration = cfAPI.Generation

	cfAPI.Status.State = v1alpha1.StateProcessing

	controllerutil.AddFinalizer(cfAPI, Finalizer)
	if !cfAPI.DeletionTimestamp.IsZero() {
		log.Info("deleting CFAPI %s/%s", cfAPI.Namespace, cfAPI.Name)
		controllerutil.RemoveFinalizer(cfAPI, Finalizer)
		return ctrl.Result{}, nil
	}

	installationConfig, err := r.compileInstallationConfig(ctx, cfAPI)
	if err != nil {
		log.Error(err, "failed to compile CFAPI installation config")
		cfAPI.Status.State = v1alpha1.StateWarning
		meta.SetStatusCondition(&cfAPI.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.ConditionTypeConfiguration,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cfAPI.Generation,
			LastTransitionTime: metav1.NewTime(time.Now()),
			Reason:             "InvalidConfiguration",
			Message:            err.Error(),
		})

		return ctrl.Result{}, k8s.NewNotReadyError().WithReason("InvalidConfiguration").WithMessage(err.Error()).WithRequeueAfter(r.requeueInterval)
	}
	meta.SetStatusCondition(&cfAPI.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ConditionTypeConfiguration,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: cfAPI.Generation,
		LastTransitionTime: metav1.NewTime(time.Now()),
		Reason:             "ValidConiguration",
	})

	cfAPI.Status.InstallationConfig = installationConfig

	installResult, err := r.installInstallables(ctx, installationConfig, installable.NewCFAPIEventRecorder(r.eventRecorder, cfAPI))
	if err != nil {
		log.Error(err, "failed to install installables")
		return ctrl.Result{}, err
	}

	log.Info("installables installed", "installResult", installResult)
	return r.applyInstallResultToStatus(installResult, cfAPI)
}

func (r *Reconciler) applyInstallResultToStatus(installResult installable.Result, cfAPI *v1alpha1.CFAPI) (ctrl.Result, error) {
	switch installResult.State {
	case installable.ResultStateSuccess:
		cfAPI.Status.State = v1alpha1.StateReady
		cfAPI.Status.URL = "https://cfapi." + cfAPI.Status.InstallationConfig.CFDomain

		meta.SetStatusCondition(&cfAPI.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.ConditionTypeInstallation,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: cfAPI.Generation,
			LastTransitionTime: metav1.NewTime(time.Now()),
			Reason:             "InstallationSuccess",
		})

		return ctrl.Result{}, nil
	case installable.ResultStateFailed:
		cfAPI.Status.URL = ""
		cfAPI.Status.State = v1alpha1.StateError
		meta.SetStatusCondition(&cfAPI.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.ConditionTypeInstallation,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cfAPI.Generation,
			LastTransitionTime: metav1.NewTime(time.Now()),
			Reason:             "InstallationFailed",
			Message:            installResult.Message,
		})

		return ctrl.Result{}, nil
	default:
		cfAPI.Status.URL = ""
		cfAPI.Status.State = v1alpha1.StateProcessing
		meta.SetStatusCondition(&cfAPI.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.ConditionTypeInstallation,
			Status:             metav1.ConditionUnknown,
			ObservedGeneration: cfAPI.Generation,
			LastTransitionTime: metav1.NewTime(time.Now()),
			Reason:             "InstallationInProgress",
			Message:            installResult.Message,
		})

		return ctrl.Result{RequeueAfter: r.requeueInterval}, nil
	}
}

func (r *Reconciler) compileInstallationConfig(ctx context.Context, cfAPI *v1alpha1.CFAPI) (v1alpha1.InstallationConfig, error) {
	aphaGWAPIEnabled, err := r.kymaClient.Istio.IsAplhaGatewayAPIEnabled(ctx)
	if err != nil {
		return v1alpha1.InstallationConfig{}, err
	}

	if !aphaGWAPIEnabled {
		return v1alpha1.InstallationConfig{}, errors.New("alpha gateway API feature is not enabled in istio. To fix this, enable the `experimental` channel on the istio module and set `spec.experimental.pilot.enableAlphaGatewayAPI` to `true` on the `kyma-system/default` Istio resource")
	}

	rootNs := cfAPI.Spec.RootNamespace
	if rootNs == "" {
		rootNs = "cf"
	}

	registrySecretName, registryURL, err := r.ensureContainerRegistry(ctx, cfAPI)
	if err != nil {
		return v1alpha1.InstallationConfig{}, err
	}

	containerRepositoryPrefix := registryURL + "/"
	if cfAPI.Spec.ContainerRepositoryPrefix != "" {
		containerRepositoryPrefix = cfAPI.Spec.ContainerRepositoryPrefix
	}

	builderRepository := registryURL + "/cfapi/kpack-builder"
	if cfAPI.Spec.BuilderRepository != "" {
		builderRepository = cfAPI.Spec.BuilderRepository
	}

	kymaDomain, err := r.kymaClient.Domain.Get(ctx)
	if err != nil {
		return v1alpha1.InstallationConfig{}, err
	}

	uaaURL, err := r.computeUaaURL(ctx, cfAPI)
	if err != nil {
		return v1alpha1.InstallationConfig{}, err
	}

	cfAdmins, err := r.computeCFAdmins(ctx, cfAPI)
	if err != nil {
		return v1alpha1.InstallationConfig{}, err
	}

	korifiIngressHost, err := r.computeKorifiIngressHost(ctx)
	if err != nil {
		return v1alpha1.InstallationConfig{}, err
	}

	return v1alpha1.InstallationConfig{
		RootNamespace:             rootNs,
		ContainerRegistrySecret:   registrySecretName,
		ContainerRepositoryPrefix: containerRepositoryPrefix,
		ContainerRegistryURL:      registryURL,
		BuilderRepository:         builderRepository,
		CFDomain:                  kymaDomain,
		UAAURL:                    uaaURL,
		CFAdmins:                  cfAdmins,
		KorifiIngressHost:         korifiIngressHost,
		UseSelfSignedCertificates: cfAPI.Spec.UseSelfSignedCertificates,
		DisableContainerRegistrySecretPropagation: cfAPI.Spec.DisableContainerRegistrySecretPropagation,
	}, nil
}

func (r *Reconciler) computeUaaURL(ctx context.Context, cfAPI *v1alpha1.CFAPI) (string, error) {
	if cfAPI.Spec.UAA != "" {
		return cfAPI.Spec.UAA, nil
	}

	return r.kymaClient.UAA.GetURL(ctx)
}

func (r *Reconciler) computeCFAdmins(ctx context.Context, cfAPI *v1alpha1.CFAPI) ([]string, error) {
	if len(cfAPI.Spec.CFAdmins) > 0 {
		return cfAPI.Spec.CFAdmins, nil
	}

	adminSubjects, err := r.kymaClient.Users.GetClusterAdmins(ctx)
	if err != nil {
		return nil, err
	}

	return slices.Collect(it.Map(slices.Values(adminSubjects), func(s rbacv1.Subject) string {
		return s.Name
	})), nil
}

func (r *Reconciler) computeKorifiIngressHost(ctx context.Context) (string, error) {
	korifiIngressService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "korifi-gateway",
			Name:      "korifi-istio",
		},
	}

	err := r.k8sClient.Get(ctx, client.ObjectKeyFromObject(korifiIngressService), korifiIngressService)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return "", err
		}

		return "", nil
	}

	if len(korifiIngressService.Status.LoadBalancer.Ingress) == 0 {
		return "", nil
	}

	hostname := korifiIngressService.Status.LoadBalancer.Ingress[0].Hostname
	if hostname == "" {
		hostname = korifiIngressService.Status.LoadBalancer.Ingress[0].IP
	}

	return hostname, nil
}

func (r *Reconciler) installInstallables(ctx context.Context, config v1alpha1.InstallationConfig, eventRecorder installable.EventRecorder) (installable.Result, error) {
	results := []installable.Result{}

	for _, inst := range r.installables {
		result, err := inst.Install(ctx, config, eventRecorder)
		if err != nil {
			return installable.Result{}, err
		}
		results = append(results, result)
	}

	slices.SortStableFunc(results, func(r1, r2 installable.Result) int {
		return int(r2.State) - int(r1.State)
	})

	return results[0], nil
}

func (r *Reconciler) ensureContainerRegistry(ctx context.Context, cfAPI *v1alpha1.CFAPI) (string, string, error) {
	if cfAPI.Spec.ContainerRegistrySecret != "" {
		customSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cfAPI.Namespace,
				Name:      cfAPI.Spec.ContainerRegistrySecret,
			},
		}

		err := r.k8sClient.Get(ctx, client.ObjectKeyFromObject(customSecret), customSecret)
		if err != nil {
			return "", "", err
		}

		registryConfig, err := r.docker.GetRegistryConfig(ctx, cfAPI.Namespace, cfAPI.Spec.ContainerRegistrySecret)
		if err != nil {
			return "", "", err
		}

		registryURLs := slices.Collect(maps.Keys(registryConfig.Auths))
		if len(registryURLs) == 0 {
			return "", "", fmt.Errorf("container registry secret %s does not specify container registries", cfAPI.Spec.ContainerRegistrySecret)
		}

		return customSecret.Name, registryURLs[0], nil
	}

	kymaRegistrySecret, err := r.kymaClient.ContainerRegistry.GetRegistrySecret(ctx, cfAPI.Namespace)
	if err != nil {
		return "", "", err
	}

	kymaRegistryURL, err := r.kymaClient.ContainerRegistry.GetRegistryURL(ctx, cfAPI.Namespace)
	if err != nil {
		return "", "", err
	}

	return kymaRegistrySecret.Name, kymaRegistryURL, nil
}
