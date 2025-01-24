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

package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"text/template"
	"time"

	"helm.sh/helm/v3/pkg/chart/loader"

	"sigs.k8s.io/controller-runtime/pkg/scheme"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	errors2 "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
)

const (
	defaultUaaUrl = "https://uaa.cf.eu10.hana.ondemand.com"
)

// CFAPIReconciler reconciles a Sample object.
type CFAPIReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	*rest.Config
	// EventRecorder for creating k8s events
	record.EventRecorder
	FinalState         v1alpha1.State
	FinalDeletionState v1alpha1.State
}

type ManifestResources struct {
	Items []*unstructured.Unstructured
	Blobs [][]byte
}

type DockerRegistryConfig struct {
	Auths map[string]DockerRegistryAuth `json:"auths"`
}

type DockerRegistryAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ContainerRegistry struct {
	Server string
	User   string
	Pass   string
}

var (
	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: v1alpha1.GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

var (
	DefaultTwuniUser = "user"
	DefaultTwuniPass = "pass"
)

func init() { //nolint:gochecknoinits
	SchemeBuilder.Register(&v1alpha1.CFAPI{}, &v1alpha1.CFAPIList{})
}

// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=cfapi,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=cfapi/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=cfapi/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch;get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=create;patch;delete
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=create;patch;delete

// SetupWithManager sets up the controller with the Manager.
func (r *CFAPIReconciler) SetupWithManager(mgr ctrl.Manager, rateLimiter RateLimiter) error {
	r.Config = mgr.GetConfig()

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.CFAPI{}).
		WithOptions(controller.Options{
			RateLimiter: TemplateRateLimiter(
				rateLimiter.BaseDelay,
				rateLimiter.FailureMaxDelay,
				rateLimiter.Frequency,
				rateLimiter.Burst,
			),
		}).
		Complete(r)
}

// Reconcile is the entry point from the controller-runtime framework.
// It performs a reconciliation based on the passed ctrl.Request object.
func (r *CFAPIReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	objectInstance := v1alpha1.CFAPI{}

	if err := r.Client.Get(ctx, req.NamespacedName, &objectInstance); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		logger.Info(req.NamespacedName.String() + " got deleted!")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// check if deletionTimestamp is set, retry until it gets deleted
	status := getStatusFromSample(&objectInstance)

	// set state to FinalDeletionState (default is Deleting) if not set for an object with deletion timestamp
	if !objectInstance.GetDeletionTimestamp().IsZero() && status.State != r.FinalDeletionState {
		return ctrl.Result{}, r.setStatusForObjectInstance(ctx, &objectInstance, status.WithState(r.FinalDeletionState))
	}

	if objectInstance.GetDeletionTimestamp().IsZero() {
		// add finalizer if not present
		if controllerutil.AddFinalizer(&objectInstance, finalizer) {
			return ctrl.Result{}, r.ssa(ctx, &objectInstance)
		}
	}

	switch status.State {
	case "":
		return ctrl.Result{}, r.HandleInitialState(ctx, &objectInstance)
	case v1alpha1.StateProcessing:
		return ctrl.Result{Requeue: true}, r.HandleProcessingState(ctx, &objectInstance)
	case v1alpha1.StateDeleting:
		return ctrl.Result{Requeue: true}, r.HandleDeletingState(ctx, &objectInstance)
	case v1alpha1.StateError:
		return ctrl.Result{Requeue: true}, r.HandleErrorState(ctx, &objectInstance)
	case v1alpha1.StateReady, v1alpha1.StateWarning:
		return ctrl.Result{RequeueAfter: requeueInterval}, r.HandleReadyState(ctx, &objectInstance)
	}

	return ctrl.Result{}, nil
}

// HandleInitialState bootstraps state handling for the reconciled resource.
func (r *CFAPIReconciler) HandleInitialState(ctx context.Context, objectInstance *v1alpha1.CFAPI) error {
	status := getStatusFromSample(objectInstance)

	return r.setStatusForObjectInstance(ctx, objectInstance, status.
		WithState(v1alpha1.StateProcessing).
		WithInstallConditionStatus(metav1.ConditionUnknown, objectInstance.GetGeneration()))
}

// HandleProcessingState processes the reconciled resource by processing the underlying resources.
// Based on the processing either a success or failure state is set on the reconciled resource.
func (r *CFAPIReconciler) HandleProcessingState(ctx context.Context, objectInstance *v1alpha1.CFAPI) error {
	status := getStatusFromSample(objectInstance)

	url, err := r.processResources(ctx, objectInstance)

	if err != nil {
		// stay in Processing state if FinalDeletionState is set to Processing
		if !objectInstance.GetDeletionTimestamp().IsZero() && r.FinalDeletionState == v1alpha1.StateProcessing {
			return nil
		}

		r.EventRecorder.Event(objectInstance, "Warning", "ResourcesInstall", err.Error())
		return r.setStatusForObjectInstance(ctx, objectInstance, status.
			WithState(v1alpha1.StateError).
			WithInstallConditionStatus(metav1.ConditionFalse, objectInstance.GetGeneration()))
	}
	// set eventual state to Ready - if no errors were found
	return r.setStatusForObjectInstance(ctx, objectInstance, status.
		WithState(r.FinalState).
		WithURL(url).
		WithInstallConditionStatus(metav1.ConditionTrue, objectInstance.GetGeneration()))
}

// HandleErrorState handles error recovery for the reconciled resource.
func (r *CFAPIReconciler) HandleErrorState(ctx context.Context, objectInstance *v1alpha1.CFAPI) error {
	status := getStatusFromSample(objectInstance)
	url, err := r.processResources(ctx, objectInstance)

	if err != nil {
		return err
	}

	// stay in Error state if FinalDeletionState is set to Error
	if !objectInstance.GetDeletionTimestamp().IsZero() && r.FinalDeletionState == v1alpha1.StateError {
		return nil
	}
	// set eventual state to Ready - if no errors were found
	return r.setStatusForObjectInstance(ctx, objectInstance, status.
		WithState(r.FinalState).
		WithURL(url).
		WithInstallConditionStatus(metav1.ConditionTrue, objectInstance.GetGeneration()))
}

// HandleDeletingState processed the deletion on the reconciled resource.
// Once the deletion if processed the relevant finalizers (if applied) are removed.
func (r *CFAPIReconciler) HandleDeletingState(ctx context.Context, objectInstance *v1alpha1.CFAPI) error {
	r.EventRecorder.Event(objectInstance, "Normal", "Deleting", "resource deleting")
	logger := log.FromContext(ctx)

	status := getStatusFromSample(objectInstance)

	// TODO
	resourceObjs, err := getResourcesFromLocalPath("", logger)

	if err != nil && controllerutil.RemoveFinalizer(objectInstance, finalizer) {
		// if error is encountered simply remove the finalizer and delete the reconciled resource
		return r.Client.Update(ctx, objectInstance)
	}
	r.EventRecorder.Event(objectInstance, "Normal", "ResourcesDelete", "deleting resources")

	// the resources to be installed are unstructured,
	// so please make sure the types are available on the target cluster
	for _, obj := range resourceObjs.Items {
		if err = r.Client.Delete(ctx, obj); err != nil && !errors2.IsNotFound(err) {
			// stay in Deleting state if FinalDeletionState is set to Deleting
			if !objectInstance.GetDeletionTimestamp().IsZero() && r.FinalDeletionState == v1alpha1.StateDeleting {
				return nil
			}

			logger.Error(err, "error during uninstallation of resources")
			r.EventRecorder.Event(objectInstance, "Warning", "ResourcesDelete", "deleting resources error")
			return r.setStatusForObjectInstance(ctx, objectInstance, status.
				WithState(v1alpha1.StateError).
				WithInstallConditionStatus(metav1.ConditionFalse, objectInstance.GetGeneration()))
		}
	}

	// if resources are ready to be deleted, remove finalizer
	if controllerutil.RemoveFinalizer(objectInstance, finalizer) {
		return r.Client.Update(ctx, objectInstance)
	}
	return nil
}

// HandleReadyState checks for the consistency of reconciled resource, by verifying the underlying resources.
func (r *CFAPIReconciler) HandleReadyState(ctx context.Context, objectInstance *v1alpha1.CFAPI) error {
	status := getStatusFromSample(objectInstance)
	if _, err := r.processResources(ctx, objectInstance); err != nil {
		// stay in Ready/Warning state if FinalDeletionState is set to Ready/Warning
		if !objectInstance.GetDeletionTimestamp().IsZero() &&
			(r.FinalDeletionState == v1alpha1.StateReady || r.FinalDeletionState == v1alpha1.StateWarning) {
			return nil
		}

		r.EventRecorder.Event(objectInstance, "Warning", "ResourcesInstall", err.Error())
		return r.setStatusForObjectInstance(ctx, objectInstance, status.
			WithState(v1alpha1.StateError).
			WithInstallConditionStatus(metav1.ConditionFalse, objectInstance.GetGeneration()))
	}
	return nil
}

func (r *CFAPIReconciler) setStatusForObjectInstance(ctx context.Context, objectInstance *v1alpha1.CFAPI,
	status *v1alpha1.CFAPIStatus,
) error {
	objectInstance.Status = *status

	if err := r.ssaStatus(ctx, objectInstance); err != nil {
		r.EventRecorder.Event(objectInstance, "Warning", "ErrorUpdatingStatus",
			fmt.Sprintf("updating state to %v", string(status.State)))
		return fmt.Errorf("error while updating status %s to: %w", status.State, err)
	}

	r.EventRecorder.Event(objectInstance, "Normal", "StatusUpdated", fmt.Sprintf("updating state to %v", string(status.State)))
	return nil
}

func (r *CFAPIReconciler) processResources(ctx context.Context, cfAPI *v1alpha1.CFAPI) (string, error) {
	logger := log.FromContext(ctx)

	r.EventRecorder.Event(cfAPI, "Normal", "ResourcesInstall", "installing resources")

	// get wildcard domain
	wildCardDomain, err := r.getWildcardDomain()
	if err != nil {
		logger.Error(err, "error getting wildcard domain")
		return "", err
	}

	cfDomain := wildCardDomain[2:]
	appsDomain := "apps." + cfDomain
	korifiApiDomain := "cfapi." + cfDomain

	logger.Info("wildcard domain retrieved : " + wildCardDomain)
	logger.Info("cf domain calculated : " + cfDomain)
	logger.Info("apps domain calculated : " + appsDomain)
	logger.Info("korifi api domain calculated : " + korifiApiDomain)

	// ensure docker registry
	logger.Info("Start ensuring docker registry ...")
	err = r.ensureDockerRegistry(ctx, cfAPI)
	if err != nil {
		logger.Error(err, "error ensuring docker registry")
		return "", err
	}
	logger.Info("docker registry ensured")

	// get app container registry
	containerRegistry, err := r.getAppContainerRegistry(ctx, cfAPI)
	if err != nil {
		logger.Error(err, "error getting app container registry")
		return "", err
	}

	// create oidc config CR if supported
	logger.Info("Starting OIDC CR creation ...")
	err = r.createOIDCConfig(ctx, cfAPI)
	if err != nil {
		logger.Error(err, "error creating OIDC CR")
		return "", err
	}
	logger.Info("OIDC CR creation completed")

	// install gateway api
	logger.Info("Installing gateway API")
	err = r.installGatewayAPI(ctx)
	if err != nil {
		logger.Error(err, "error installing gateway api")
		return "", err
	}
	logger.Info("Gateway API installed")

	// install cert manager
	err = r.installOneGlob(ctx, "./module-data/cert-manager/cert-manager.yaml")
	if err != nil {
		logger.Error(err, "error installing cert manager")
		return "", err
	}

	// create needed namespaces
	logger.Info("Start creating namespaces ...")
	err = r.createNamespaces(ctx, containerRegistry)
	if err != nil {
		logger.Error(err, "error creating namespaces")
		return "", err
	}
	logger.Info("namespaces created")

	// generate ingress certificates
	logger.Info("Start generating ingress certificates ...")
	err = r.generateIngressCertificates(ctx, cfDomain, appsDomain, korifiApiDomain)
	if err != nil {
		logger.Error(err, "problem generating ingress certificates")
		return "", err
	}
	logger.Info("ingress certificates generated")

	err = r.installOneGlob(ctx, "./module-data/kpack/release-*.yaml")
	if err != nil {
		logger.Error(err, "error installing kpack")
		return "", err
	}

	err = r.installOneGlob(ctx, "./module-data/servicebinding/servicebinding-runtime-v*.yaml")
	if err != nil {
		logger.Error(err, "error installing servicebindig runtime")
		return "", err
	}
	err = r.installOneGlob(ctx, "./module-data/servicebinding/servicebinding-workloadresourcemappings-v*.yaml")
	if err != nil {
		logger.Error(err, "error installing servicebindig workloadresourcemappings")
		return "", err
	}

	// create buildkit secret
	logger.Info("Start creating buildkit secret ...")
	err = r.createBuildkitSecret(ctx, containerRegistry)
	if err != nil {
		logger.Error(err, "error creating buildkit secret")
		return "", err
	}
	logger.Info("buildkit secret created")

	// deploy korifi
	logger.Info("Start deploying korifi ...")
	var uaaUrl = cfAPI.Spec.UAA
	if uaaUrl == "" {
		uaaUrl = defaultUaaUrl
	}
	err = r.deployKorifi(ctx, appsDomain, korifiApiDomain, cfDomain, containerRegistry.Server, uaaUrl)
	if err != nil {
		logger.Error(err, "error during deployment of Korifi")
		return "", err
	}
	logger.Info("korifi deployed")

	// create dns entries
	logger.Info("Start creating dns entries ...")
	err = r.createDNSEntries(ctx, korifiApiDomain, appsDomain)
	if err != nil {
		logger.Error(err, "error creating dns entries")
		return "", err
	}
	logger.Info("dns entries created")

	var subjects = toSubjectList(cfAPI.Spec.CFAdmins)
	err = r.assignCfAdministrators(ctx, subjects, cfAPI.Spec.RootNamespace)
	if err != nil {
		logger.Error(err, "Failed to assing CF administrators")
		return "", err
	}

	logger.Info("CFAPI reconciled successfully")

	return "https://" + korifiApiDomain, nil
}

func (r *CFAPIReconciler) ensureDockerRegistry(ctx context.Context, cfAPI *v1alpha1.CFAPI) error {
	logger := log.FromContext(ctx)

	if cfAPI.Spec.AppImagePullSecret != "" {
		logger.Info("App Container Img Reg Secret is set, using it")
		return nil
	}

	if !r.crdExists(ctx, "DockerRegistry") {
		logger.Info("DockerRegistry CRD does not exist")
		return errors.New("DockerRegistry CRD does not exist. Create it by enablib docker registry Kyma module")
	}

	err := r.installOneGlob(ctx, "./module-data/docker-registry/docker-registry.yaml")
	if err != nil {
		logger.Error(err, "error installing docker registry")
		return err
	}

	err = r.waitForSecret("cfapi-system", "dockerregistry-config-external")
	if err != nil {
		logger.Error(err, "error waiting for secret dockerregistry-config-external")
		return err
	}

	return nil
}

func (r *CFAPIReconciler) createOIDCConfig(ctx context.Context, cfAPI *v1alpha1.CFAPI) error {
	logger := log.FromContext(ctx)

	if r.crdExists(ctx, "OpenIDConnect") {
		logger.Info("OIDC CR exists, create CR")

		var uaaUrl = cfAPI.Spec.UAA
		if uaaUrl == "" {
			uaaUrl = defaultUaaUrl
		}
		vals := struct {
			UAA string
		}{
			UAA: uaaUrl,
		}

		t1 := template.New("oidcUAA")

		t2, err := t1.ParseFiles("./module-data/oidc/oidc-uaa-experimental.tmpl")

		if err != nil {
			logger.Error(err, "error during parsing of oidc template")
			return err
		}

		buf := &bytes.Buffer{}

		err = t2.ExecuteTemplate(buf, "oidcUAA", vals)

		if err != nil {
			logger.Error(err, "error during execution of oidc template")
			return err
		}

		s := buf.String()

		resourceObjs, err := parseManifestStringToObjects(s)

		if err != nil {
			logger.Error(err, "error during parsing of twuni tls route")
			return nil
		}

		for _, obj := range resourceObjs.Items {
			if err = r.ssa(ctx, obj); err != nil && !errors2.IsAlreadyExists(err) {
				logger.Error(err, "error during installation of twuni tls route")
				return err
			}
		}
	} else {
		logger.Info("OIDC CR does not exist, skip creating CR")
	}

	return nil
}

func (r *CFAPIReconciler) getAppContainerRegistry(ctx context.Context, cfAPI *v1alpha1.CFAPI) (ContainerRegistry, error) {
	logger := log.FromContext(ctx)

	if cfAPI.Spec.AppImagePullSecret != "" {
		logger.Info("App Container Img Reg Secret is set, using it")
		// extract container registry from secret
		secret := corev1.Secret{}
		err := r.Client.Get(context.Background(), client.ObjectKey{
			Namespace: "korifi",
			Name:      cfAPI.Spec.AppImagePullSecret,
		}, &secret)

		if err != nil {
			logger.Error(err, "error getting app container registry secret")
			return ContainerRegistry{}, err
		}

		return ContainerRegistry{
			Server: string(secret.Data["server"]),
			User:   string(secret.Data["username"]),
			Pass:   string(secret.Data["password"]),
		}, nil
	}

	logger.Info("Constructing app container registry from dockerregistry-config-external secret ")

	secret := corev1.Secret{}
	err := r.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "cfapi-system",
		Name:      "dockerregistry-config-external",
	}, &secret)

	if err != nil {
		logger.Error(err, "error getting app container registry secret")
		return ContainerRegistry{}, err
	}

	return ContainerRegistry{
		Server: string(secret.Data["pushRegAddr"]),
		User:   string(secret.Data["username"]),
		Pass:   string(secret.Data["password"]),
	}, nil
}

func (r *CFAPIReconciler) createDNSEntries(ctx context.Context, korifiAPI, appsDomain string) error {
	logger := log.FromContext(ctx)

	// get ingress hostname
	ingress := corev1.Service{}
	err := r.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "korifi-gateway",
		Name:      "korifi-istio",
	}, &ingress)

	if err != nil {
		logger.Error(err, "error getting ingress hostname")
		return err
	}

	hostname := ingress.Status.LoadBalancer.Ingress[0].Hostname

	// create dns entries
	vals := struct {
		KorifiAPI   string
		IngressHost string
		AppsDomain  string
	}{
		KorifiAPI:   korifiAPI,
		IngressHost: hostname,
		AppsDomain:  appsDomain,
	}

	t1 := template.New("dnsEntries")
	t2, err := t1.ParseFiles("./module-data/dns-entries/dns-entries.tmpl")
	if err != nil {
		logger.Error(err, "error during parsing of dns entries template")
		return err
	}

	buf := &bytes.Buffer{}

	err = t2.ExecuteTemplate(buf, "dnsEntries", vals)
	if err != nil {
		logger.Error(err, "error during execution of dns entries template")
		return err
	}

	s := buf.String()

	resourceObjs, err := parseManifestStringToObjects(s)

	if err != nil {
		logger.Error(err, "error during parsing of dns entries")
		return nil
	}

	for _, obj := range resourceObjs.Items {
		if err = r.ssa(ctx, obj); err != nil && !errors2.IsAlreadyExists(err) {
			logger.Error(err, "error during installation of dns entries")
			return err
		}
	}

	return nil
}

func (r *CFAPIReconciler) createBuildkitSecret(ctx context.Context, appContainerRegistry ContainerRegistry) error {
	logger := log.FromContext(ctx)

	secretExists := r.secretExists("cfapi-system", "buildkit")

	if secretExists {
		logger.Info("buildkit secret already exists, patching it")

		err := r.patchDockerSecret(ctx, "buildkit", "cfapi-system", appContainerRegistry.Server,
			appContainerRegistry.User, appContainerRegistry.Pass)

		if err != nil {
			logger.Error(err, "error patching buildkit secret")
			return err
		}
	} else {
		err := r.createDockerSecret(ctx, "buildkit", "cfapi-system", appContainerRegistry.Server,
			appContainerRegistry.User, appContainerRegistry.Pass)

		if err != nil {
			logger.Error(err, "error creating buildkit secret")
			return err
		}
	}

	return nil
}

func (r *CFAPIReconciler) createDockerSecret(ctx context.Context, name, namespace, server, username, password string) error {
	logger := log.FromContext(ctx)

	conf := DockerRegistryConfig{
		Auths: map[string]DockerRegistryAuth{},
	}

	conf.Auths[server] = DockerRegistryAuth{
		Username: username,
		Password: password,
	}

	secretData, err := json.Marshal(conf)

	if err != nil {
		logger.Error(err, "error marshalling docker registry config")
		return err
	}

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type:       "kubernetes.io/dockerconfigjson",
		StringData: map[string]string{".dockerconfigjson": string(secretData)},
	}

	err = r.Client.Create(context.Background(), &secret)

	if err != nil {
		logger.Error(err, "error creating "+name+" secret in ns "+namespace)
		return err
	}

	return nil
}

func (r *CFAPIReconciler) generateIngressCertificates(ctx context.Context, cfDomain, appsDomain, korifiApiDomain string) error {
	logger := log.FromContext(ctx)

	vals := struct {
		CFDomain        string
		AppsDomain      string
		KorifiAPIDomain string
	}{
		CFDomain:        cfDomain,
		AppsDomain:      appsDomain,
		KorifiAPIDomain: korifiApiDomain,
	}

	t1 := template.New("ingressCerts")
	t2, err := t1.ParseFiles("./module-data/ingress-certificates/ingress-certificates.tmpl")

	if err != nil {
		logger.Error(err, "error during parsing of ingress certificates template")
		return err
	}

	buf := &bytes.Buffer{}

	err = t2.ExecuteTemplate(buf, "ingressCerts", vals)

	if err != nil {
		logger.Error(err, "error during execution of ingress certificates template")
		return err
	}

	s := buf.String()

	resourceObjs, err := parseManifestStringToObjects(s)

	if err != nil {
		logger.Error(err, "error during parsing of ingress certificates")
		return nil
	}

	for _, obj := range resourceObjs.Items {
		if err = r.ssa(ctx, obj); err != nil && !errors2.IsAlreadyExists(err) {
			logger.Error(err, "error during installation of cert manager resources")
			return err
		}
	}

	// wait for respective secrets to be created
	err = r.waitForSecret("korifi", "korifi-api-ingress-cert")
	if err != nil {
		logger.Error(err, "error waiting for secret korifi-api-ingress-cert")
		return err
	}

	err = r.waitForSecret("korifi", "korifi-workloads-ingress-cert")
	if err != nil {
		logger.Error(err, "error waiting for secret korifi-workloads-ingress-cert")
		return err
	}

	return nil
}

func (r *CFAPIReconciler) waitForSecret(namespace, name string) error {
	logger := log.FromContext(context.Background())

	start := time.Now()

	for {
		secretExists := r.secretExists(namespace, name)

		if secretExists {
			logger.Info("secret " + name + " found")
			break
		}

		logger.Info("secret " + name + " not found, retrying...")
		time.Sleep(1 * time.Minute)

		now := time.Now()

		diff := now.Sub(start)

		if diff.Minutes() > 15 {
			return errors.New("timeout waiting for secret " + name)
		}
	}

	return nil
}

func (r *CFAPIReconciler) getWildcardDomain() (string, error) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "networking.istio.io",
		Kind:    "Gateway",
		Version: "v1beta1",
	})

	err := r.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "kyma-system",
		Name:      "kyma-gateway",
	}, u)

	if err != nil {
		return "", err
	}

	uc := u.UnstructuredContent()

	servers, found, err := unstructured.NestedSlice(uc, "spec", "servers")

	if err != nil {
		return "", err
	}

	if !found {
		return "", errors.New("wildcard domain field not found")
	}

	s := servers[0].(map[string]interface{})["hosts"].([]interface{})[0].(string)

	return s, nil
}

func (r *CFAPIReconciler) createNamespaces(ctx context.Context, appContainerRegistry ContainerRegistry) error {
	logger := log.FromContext(ctx)

	err := r.installOneGlob(ctx, "./module-data/namespaces/namespaces.yaml")
	if err != nil {
		logger.Error(err, "error creating namespaces")
		return err
	}

	// create container image pull secrets in namespaces
	namespaces := []string{"korifi", "cf"}
	for _, ns := range namespaces {

		secretExists := r.secretExists(ns, "cfapi-app-registry")

		if secretExists {
			logger.Info("image pull secret already exists in ns " + ns + ", patching it")

			err = r.patchDockerSecret(ctx, "cfapi-app-registry", ns, appContainerRegistry.Server,
				appContainerRegistry.User, appContainerRegistry.Pass)

			if err != nil {
				logger.Error(err, "error patching image pull secret")
				return err
			}
		} else {
			err = r.createDockerSecret(ctx, "cfapi-app-registry", ns, appContainerRegistry.Server,
				appContainerRegistry.User, appContainerRegistry.Pass)

			if err != nil {
				logger.Error(err, "error creating image pull secret")
				return err
			}
		}
	}

	return nil
}

func (r *CFAPIReconciler) installGatewayAPI(ctx context.Context) error {
	logger := log.FromContext(ctx)

	err := r.installOneGlob(ctx, "./module-data/gateway-api/experimental-install.yaml")
	if err != nil {
		logger.Error(err, "error installing gateway API")
		return err
	}

	deploy := &appsv1.Deployment{}
	err = r.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "istio-system",
		Name:      "istiod",
	}, deploy)

	if err != nil {
		logger.Error(err, "error getting istiod deployment")
		return err
	}

	envVarFound := false

	for _, env := range deploy.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "PILOT_ENABLE_ALPHA_GATEWAY_API" {
			envVarFound = true
		}
	}

	if !envVarFound {
		deploy.Spec.Template.Spec.Containers[0].Env = append(deploy.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  "PILOT_ENABLE_ALPHA_GATEWAY_API",
			Value: "true",
		})

		if err = r.ssa(ctx, deploy); err != nil && !errors2.IsAlreadyExists(err) {
			logger.Error(err, "error during patching istiod deployment")
			return err
		}
	}

	err = r.installOneGlob(ctx, "./module-data/envoy-filter/empty-envoy-filter.yaml")
	if err != nil {
		logger.Error(err, "error installing envoy filter")
		return err
	}

	return nil
}

func getStatusFromSample(objectInstance *v1alpha1.CFAPI) v1alpha1.CFAPIStatus {
	return objectInstance.Status
}

// Korifi HELM chart deployment
func (r *CFAPIReconciler) deployKorifi(ctx context.Context, appsDomain, korifiAPIDomain, cfDomain, crDomain, uaaURL string) error {
	logger := log.FromContext(ctx)

	helmfile, err := findOneGlob("./module-data/korifi/korifi-*.tgz")
	if err != nil {
		logger.Error(err, "Failed to find korifi helm chart under dir module-data/korifi")
		return err
	}
	chart, err := loader.Load(helmfile)
	if err != nil {
		logger.Error(err, "error loading korifi helm chart")
		return err
	}

	values, err := loadOneYaml("./module-data/korifi/values-*.yaml")
	if err != nil {
		logger.Error(err, "Failed to load korifi helm chart release values")
		return err
	}

	values_cfapi, err := loadOneYaml("./module-data/korifi/values.yaml")
	if err != nil {
		logger.Error(err, "Failed to load CFAPI values for korifi helm chart")
		return err
	}

	DeepUpdate(values, values_cfapi)

	values_dynamic := map[string]interface{}{
		"api": map[string]interface{}{
			"apiServer": map[string]interface{}{
				"url": korifiAPIDomain,
			},
			"logcache": map[string]interface{}{
				"url": "logcache." + cfDomain,
			},
			"uaaURL": uaaURL,
		},
		"kpackImageBuilder": map[string]interface{}{
			"builderRepository": crDomain + "/trinity/kpack-builder",
		},
		"containerRepositoryPrefix": crDomain + "/",
		"defaultAppDomainName":      appsDomain,
		"cfDomain":                  cfDomain,
	}

	DeepUpdate(values, values_dynamic)

	err = applyRelease(chart, "korifi", "korifi", values, logger)

	return err
}
