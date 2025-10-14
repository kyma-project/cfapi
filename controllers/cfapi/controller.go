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

	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/go-logr/logr"
	v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/registry"
	"github.com/kyma-project/cfapi/tools/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
)

const (
	Finalizer = "cfapi.kyma-project.io/finalizer"
)

type Reconciler struct {
	k8sClient    client.Client
	scheme       *runtime.Scheme
	kymaRegistry *registry.Kyma
}

func NewReconciler(
	k8sClient client.Client,
	scheme *runtime.Scheme,
	kymaRegistry *registry.Kyma,
	log logr.Logger,
) *k8s.PatchingReconciler[v1alpha1.CFAPI] {
	apiReconciler := &Reconciler{
		k8sClient:    k8sClient,
		scheme:       scheme,
		kymaRegistry: kymaRegistry,
	}
	return k8s.NewPatchingReconciler(ctrl.Log, k8sClient, apiReconciler)
}

// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=cfapi,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=cfapi/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=cfapi/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch;get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=create;patch;delete
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=create;patch;delete

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) *builder.Builder {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.CFAPI{})
}

func (r *Reconciler) ReconcileResource(ctx context.Context, cfAPI *v1alpha1.CFAPI) (ctrl.Result, error) {
	cfAPI.Status.ObservedGeneration = cfAPI.Generation

	cfAPI.Status.State = v1alpha1.StateProcessing

	controllerutil.AddFinalizer(cfAPI, Finalizer)
	if !cfAPI.DeletionTimestamp.IsZero() {
		controllerutil.RemoveFinalizer(cfAPI, Finalizer)
	}

	err := r.ensureContainerRegistrySecret(ctx, cfAPI)
	if err != nil {
		cfAPI.Status.State = v1alpha1.StateWarning
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) ensureContainerRegistrySecret(ctx context.Context, cfAPI *v1alpha1.CFAPI) error {
	if cfAPI.Spec.AppImagePullSecret != "" {
		customSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cfAPI.Namespace,
				Name:      cfAPI.Spec.AppImagePullSecret,
			},
		}

		err := r.k8sClient.Get(ctx, client.ObjectKeyFromObject(customSecret), customSecret)
		if err != nil {
			return err
		}
		cfAPI.Status.ContainerRegistrySecret = customSecret.Name
		return nil
	}

	kymaRegistrySecret, err := r.kymaRegistry.GetRegistrySecret(ctx, cfAPI.Namespace)
	if err != nil {
		return err
	}

	cfAPI.Status.ContainerRegistrySecret = kymaRegistrySecret.Name
	return nil
}
