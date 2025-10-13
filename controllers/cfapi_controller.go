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
	"strings"
	"text/template"
	"time"

	"helm.sh/helm/v3/pkg/chart/loader"

	"sigs.k8s.io/controller-runtime/pkg/scheme"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
)

var (
	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: v1alpha1.GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func init() { //nolint:gochecknoinits
	SchemeBuilder.Register(&v1alpha1.CFAPI{}, &v1alpha1.CFAPIList{})
}

const (
	kymaSystemNamespace          = "kyma-system"
	btpServiceOperatorSecretName = "sap-btp-service-operator"

	uaaURLPrefix = "https://uaa.cf."

	finalState         = v1alpha1.StateReady
	finalDeletionState = v1alpha1.StateDeleting
)

// CFAPIReconciler reconciles a Sample object.
type CFAPIReconciler struct {
	k8sClient     client.Client
	eventRecorder record.EventRecorder
	scheme        *runtime.Scheme
}

func NewCFApiReconciler(
	k8sClient client.Client,
	eventRecorder record.EventRecorder,
	scheme *runtime.Scheme,
) *CFAPIReconciler {
	return &CFAPIReconciler{
		k8sClient:     k8sClient,
		eventRecorder: eventRecorder,
		scheme:        scheme,
	}
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

// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=cfapi,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=cfapi/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=cfapi/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch;get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=create;patch;delete
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=create;patch;delete

// SetupWithManager sets up the controller with the Manager.
func (r *CFAPIReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.CFAPI{}).
		Complete(r)
}

// Reconcile is the entry point from the controller-runtime framework.
// It performs a reconciliation based on the passed ctrl.Request object.
func (r *CFAPIReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	objectInstance := v1alpha1.CFAPI{}

	if err := r.k8sClient.Get(ctx, req.NamespacedName, &objectInstance); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		logger.Info(req.String() + " got deleted!")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// check if deletionTimestamp is set, retry until it gets deleted
	status := getStatusFromSample(&objectInstance)

	// set state to FinalDeletionState (default is Deleting) if not set for an object with deletion timestamp
	if !objectInstance.GetDeletionTimestamp().IsZero() {
		return ctrl.Result{}, r.setStatusForObjectInstance(ctx, &objectInstance, status.WithState(finalDeletionState))
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
		if !objectInstance.GetDeletionTimestamp().IsZero() {
			return nil
		}

		r.eventRecorder.Event(objectInstance, "Warning", "ResourcesInstall", err.Error())
		return r.setStatusForObjectInstance(ctx, objectInstance, status.
			WithState(v1alpha1.StateError).
			WithInstallConditionStatus(metav1.ConditionFalse, objectInstance.GetGeneration()))
	}
	// set eventual state to Ready - if no errors were found
	return r.setStatusForObjectInstance(ctx, objectInstance, status.
		WithState(finalState).
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

	if !objectInstance.GetDeletionTimestamp().IsZero() {
		return nil
	}
	// set eventual state to Ready - if no errors were found
	return r.setStatusForObjectInstance(ctx, objectInstance, status.
		WithState(finalState).
		WithURL(url).
		WithInstallConditionStatus(metav1.ConditionTrue, objectInstance.GetGeneration()))
}

// HandleDeletingState processed the deletion on the reconciled resource.
// Once the deletion if processed the relevant finalizers (if applied) are removed.
func (r *CFAPIReconciler) HandleDeletingState(ctx context.Context, objectInstance *v1alpha1.CFAPI) error {
	r.eventRecorder.Event(objectInstance, "Normal", "Deleting", "resource deleting")
	logger := log.FromContext(ctx)

	status := getStatusFromSample(objectInstance)

	// TODO
	resourceObjs, err := getResourcesFromLocalPath("", logger)

	if err != nil && controllerutil.RemoveFinalizer(objectInstance, finalizer) {
		// if error is encountered simply remove the finalizer and delete the reconciled resource
		return r.k8sClient.Update(ctx, objectInstance)
	}
	r.eventRecorder.Event(objectInstance, "Normal", "ResourcesDelete", "deleting resources")

	// the resources to be installed are unstructured,
	// so please make sure the types are available on the target cluster
	for _, obj := range resourceObjs.Items {
		if err = r.k8sClient.Delete(ctx, obj); err != nil && !k8serrors.IsNotFound(err) {
			if !objectInstance.GetDeletionTimestamp().IsZero() {
				return nil
			}

			logger.Error(err, "error during uninstallation of resources")
			r.eventRecorder.Event(objectInstance, "Warning", "ResourcesDelete", "deleting resources error")
			return r.setStatusForObjectInstance(ctx, objectInstance, status.
				WithState(v1alpha1.StateError).
				WithInstallConditionStatus(metav1.ConditionFalse, objectInstance.GetGeneration()))
		}
	}

	// if resources are ready to be deleted, remove finalizer
	if controllerutil.RemoveFinalizer(objectInstance, finalizer) {
		return r.k8sClient.Update(ctx, objectInstance)
	}
	return nil
}

// HandleReadyState checks for the consistency of reconciled resource, by verifying the underlying resources.
func (r *CFAPIReconciler) HandleReadyState(ctx context.Context, objectInstance *v1alpha1.CFAPI) error {
	status := getStatusFromSample(objectInstance)
	if _, err := r.processResources(ctx, objectInstance); err != nil {
		if !objectInstance.GetDeletionTimestamp().IsZero() {
			return nil
		}

		r.eventRecorder.Event(objectInstance, "Warning", "ResourcesInstall", err.Error())
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
		r.eventRecorder.Event(objectInstance, "Warning", "ErrorUpdatingStatus",
			fmt.Sprintf("updating state to %v", string(status.State)))
		return fmt.Errorf("error while updating status %s to: %w", status.State, err)
	}

	r.eventRecorder.Event(objectInstance, "Normal", "StatusUpdated", fmt.Sprintf("updating state to %v", string(status.State)))
	return nil
}

func (r *CFAPIReconciler) processResources(ctx context.Context, cfAPI *v1alpha1.CFAPI) (string, error) {
	logger := log.FromContext(ctx)

	r.eventRecorder.Event(cfAPI, "Normal", "ResourcesInstall", "installing resources")

	// get wildcard domain
	wildCardDomain, err := r.getWildcardDomain()
	if err != nil {
		return "", fmt.Errorf("failed to get wildcard domain: %w", err)
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
		return "", fmt.Errorf("failed to ensure docker registry: %w", err)
	}
	logger.Info("docker registry ensured")

	// get app container registry
	containerRegistry, err := r.getAppContainerRegistry(ctx, cfAPI)
	if err != nil {
		return "", fmt.Errorf("failed to get app container registry: %w", err)
	}

	// get uaa url
	uaaUrl := cfAPI.Spec.UAA
	if uaaUrl == "" {
		uaaUrl, err = r.retrieveUaaUrl(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get UAA URL: %w", err)
		}
	}

	// create oidc config CR if supported
	logger.Info("Starting OIDC CR creation ...")
	err = r.createOIDCConfig(ctx, uaaUrl)
	if err != nil {
		return "", fmt.Errorf("failed to create OIDC resource: %w", err)
	}
	logger.Info("OIDC CR creation completed")

	// install gateway api
	logger.Info("Installing gateway API")
	err = r.installGatewayAPI(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to install gateway API: %w", err)
	}
	logger.Info("Gateway API installed")

	// create needed namespaces
	logger.Info("Start creating namespaces ...")
	err = r.createNamespaces(ctx, containerRegistry)
	if err != nil {
		return "", fmt.Errorf("failed to create namespaces: %w", err)
	}
	logger.Info("namespaces created")

	// generate ingress certificates
	logger.Info("Start generating ingress certificates ...")
	err = r.generateCertificates(ctx, cfDomain, appsDomain, korifiApiDomain)
	if err != nil {
		return "", fmt.Errorf("failed to generate certificates: %w", err)
	}
	logger.Info("ingress certificates generated")

	err = r.installOneGlob(ctx, "./module-data/kpack/release-*.yaml")
	if err != nil {
		return "", fmt.Errorf("failed to install kpack: %w", err)
	}

	// deploy korifi
	logger.Info("Start deploying korifi ...")
	err = r.deployKorifi(ctx, appsDomain, korifiApiDomain, containerRegistry.Server, uaaUrl)
	if err != nil {
		return "", fmt.Errorf("failed to deploy Korifi: %w", err)
	}
	logger.Info("korifi deployed")

	// create dns entries
	logger.Info("Start creating dns entries ...")
	err = r.createDNSEntries(ctx, korifiApiDomain, appsDomain)
	if err != nil {
		return "", fmt.Errorf("failed to create dns entries: %w", err)
	}
	logger.Info("dns entries created")

	subjects := toSubjectList(cfAPI.Spec.CFAdmins)
	err = r.assignCfAdministrators(ctx, subjects, cfAPI.Spec.RootNamespace)
	if err != nil {
		return "", fmt.Errorf("failed to assign cf administrators: %w", err)
	}

	logger.Info("Start deploying the BTP service broker...")
	err = r.deployServiceBroker(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to deploy BTP Service Broker: %w", err)
	}
	logger.Info("BTP service broker deployed")

	logger.Info("CFAPI reconciled successfully")

	return "https://" + korifiApiDomain, nil
}

func (r *CFAPIReconciler) deployServiceBroker(ctx context.Context) error {
	logger := log.FromContext(ctx)

	chart, err := loader.Load("./module-data/btp-service-broker/helm")
	if err != nil {
		return fmt.Errorf("failed to load the BTP broker helm chart: %w", err)
	}

	return applyRelease(chart, "cfapi-system", "btp-service-broker", map[string]any{}, logger)
}

func (r *CFAPIReconciler) ensureDockerRegistry(ctx context.Context, cfAPI *v1alpha1.CFAPI) error {
	logger := log.FromContext(ctx)

	if cfAPI.Spec.AppImagePullSecret != "" {
		logger.Info("App Container Img Reg Secret is set, using it")
		return nil
	}

	dockerRegistryCRDExists, err := r.crdExists(ctx, "DockerRegistry")
	if err != nil {
		return fmt.Errorf("failed to check whether the DockerRegistry custom resource exists: %w", err)
	}

	if !dockerRegistryCRDExists {
		return fmt.Errorf("DockerRegistry CRD does not exist. Create it by enabling docker registry Kyma module")
	}

	err = r.installOneGlob(ctx, "./module-data/docker-registry/docker-registry.yaml")
	if err != nil {
		return fmt.Errorf("failed to install docker registry: %w", err)
	}

	err = r.waitForSecret("cfapi-system", "dockerregistry-config-external")
	if err != nil {
		return fmt.Errorf("failed awaiting for the docker registry external secret: %w", err)
	}

	return nil
}

func (r *CFAPIReconciler) createOIDCConfig(ctx context.Context, uaaURL string) error {
	logger := log.FromContext(ctx)

	oidcCrdExists, err := r.crdExists(ctx, "OpenIDConnect")
	if err != nil {
		return fmt.Errorf("failed to check whether the OpenIDConnect custom resource definition exists: %w", err)
	}

	if !oidcCrdExists {
		logger.Info("OpenIDConnect CRD does not exist, skip creating the resource")
		return nil
	}

	logger.Info("creating OpenIDConnect resource...")

	oidcTemplate, err := template.ParseFiles("./module-data/oidc/oidc-uaa-experimental.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse the oidc template: %w", err)
	}

	vals := struct {
		UAA string
	}{
		UAA: uaaURL,
	}
	buf := &bytes.Buffer{}
	err = oidcTemplate.ExecuteTemplate(buf, oidcTemplate.Name(), vals)
	if err != nil {
		return fmt.Errorf("failed to execute the oidc template: %w", err)
	}

	resourceObjs, err := parseManifestStringToObjects(buf.String())
	if err != nil {
		return fmt.Errorf("failed to parse the oidc rendered template to objects: %w", err)
	}

	for _, obj := range resourceObjs.Items {
		if err = r.ssa(ctx, obj); client.IgnoreAlreadyExists(err) != nil {
			return fmt.Errorf("failed to create the oidc custom resource: %w", err)
		}
	}

	return nil
}

func (r *CFAPIReconciler) getAppContainerRegistry(ctx context.Context, cfAPI *v1alpha1.CFAPI) (ContainerRegistry, error) {
	logger := log.FromContext(ctx)

	if cfAPI.Spec.AppImagePullSecret != "" {
		logger.Info("App Container Img Reg Secret is set, using it")
		// extract container registry from secret
		secret := corev1.Secret{}
		err := r.k8sClient.Get(context.Background(), client.ObjectKey{
			Namespace: "korifi",
			Name:      cfAPI.Spec.AppImagePullSecret,
		}, &secret)
		if err != nil {
			return ContainerRegistry{}, fmt.Errorf("failed to get app container registry secret: %w", err)
		}

		return ContainerRegistry{
			Server: string(secret.Data["server"]),
			User:   string(secret.Data["username"]),
			Pass:   string(secret.Data["password"]),
		}, nil
	}

	logger.Info("Constructing app container registry from dockerregistry-config-external secret ")

	secret := corev1.Secret{}
	err := r.k8sClient.Get(context.Background(), client.ObjectKey{
		Namespace: "cfapi-system",
		Name:      "dockerregistry-config-external",
	}, &secret)
	if err != nil {
		return ContainerRegistry{}, fmt.Errorf("failed to get app container registry secret: %w", err)
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
	err := r.k8sClient.Get(context.Background(), client.ObjectKey{
		Namespace: "korifi-gateway",
		Name:      "korifi-istio",
	}, &ingress)
	if err != nil {
		return fmt.Errorf("failed to get ingress service: %w", err)
	}

	hostname := ingress.Status.LoadBalancer.Ingress[0].Hostname

	if hostname == "" {
		logger.Info("hostname not found in ingress service, will try to use IP")
		hostname = ingress.Status.LoadBalancer.Ingress[0].IP
	}

	log.Log.Info("hostname to use for dns entries: " + hostname)

	dnsEntriesTemplate, err := template.ParseFiles("./module-data/dns-entries/dns-entries.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse dns entries template: %w", err)
	}

	vals := struct {
		KorifiAPI   string
		IngressHost string
		AppsDomain  string
	}{
		KorifiAPI:   korifiAPI,
		IngressHost: hostname,
		AppsDomain:  appsDomain,
	}
	buf := &bytes.Buffer{}
	err = dnsEntriesTemplate.ExecuteTemplate(buf, dnsEntriesTemplate.Name(), vals)
	if err != nil {
		return fmt.Errorf("failed to execute the dns entries template: %w", err)
	}

	s := buf.String()
	dnsEntries, err := parseManifestStringToObjects(s)
	if err != nil {
		return fmt.Errorf("failed to parse dns entries rendered template to objects: %w", err)
	}

	for _, obj := range dnsEntries.Items {
		if err = r.ssa(ctx, obj); client.IgnoreAlreadyExists(err) != nil {
			return fmt.Errorf("failed to create dns entries: %w", err)
		}
	}

	return nil
}

func (r *CFAPIReconciler) createDockerSecret(ctx context.Context, name, namespace, server, username, password string) error {
	conf := DockerRegistryConfig{
		Auths: map[string]DockerRegistryAuth{},
	}

	conf.Auths[server] = DockerRegistryAuth{
		Username: username,
		Password: password,
	}

	secretData, err := json.Marshal(conf)
	if err != nil {
		return fmt.Errorf("failed to marshal docker registry config: %w", err)
	}

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type:       "kubernetes.io/dockerconfigjson",
		StringData: map[string]string{".dockerconfigjson": string(secretData)},
	}

	err = r.k8sClient.Create(ctx, &secret)
	if err != nil {
		return fmt.Errorf("failed to create secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}

	return nil
}

func (r *CFAPIReconciler) generateCertificates(ctx context.Context, cfDomain, appsDomain, korifiApiDomain string) error {
	certTemplate, err := template.ParseFiles("./module-data/certificates/certificates.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse certificates template: %w", err)
	}

	vals := struct {
		CFDomain        string
		AppsDomain      string
		KorifiAPIDomain string
	}{
		CFDomain:        cfDomain,
		AppsDomain:      appsDomain,
		KorifiAPIDomain: korifiApiDomain,
	}
	buf := &bytes.Buffer{}
	err = certTemplate.ExecuteTemplate(buf, certTemplate.Name(), vals)
	if err != nil {
		return fmt.Errorf("failed to execute the certificates template: %w", err)
	}

	s := buf.String()
	certs, err := parseManifestStringToObjects(s)
	if err != nil {
		return fmt.Errorf("failed to parse certificates rendered template to objects: %w", err)
	}

	for _, obj := range certs.Items {
		if err = r.ssa(ctx, obj); client.IgnoreAlreadyExists(err) != nil {
			return fmt.Errorf("failed to create certificate: %w", err)
		}
	}

	// wait for respective secrets to be created
	for _, secretName := range []string{
		"korifi-api-ingress-cert",
		"korifi-api-internal-cert",
		"korifi-workloads-ingress-cert",
		"korifi-controllers-webhook-cert",
		"korifi-kpack-image-builder-webhook-cert",
		"korifi-statefulset-runner-webhook-cert",
	} {
		err = r.waitForSecret("korifi", secretName)
		if err != nil {
			return fmt.Errorf("failed to await secret %s: %w", secretName, err)
		}
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
		time.Sleep(10 * time.Second)

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

	err := r.k8sClient.Get(context.Background(), client.ObjectKey{
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

	s := servers[0].(map[string]any)["hosts"].([]any)[0].(string)

	return s, nil
}

func (r *CFAPIReconciler) createNamespaces(ctx context.Context, appContainerRegistry ContainerRegistry) error {
	logger := log.FromContext(ctx)

	err := r.installOneGlob(ctx, "./module-data/namespaces/namespaces.yaml")
	if err != nil {
		return fmt.Errorf("failed to create namespaces: %w", err)
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
				return fmt.Errorf("failed to patck image pull secret: %w", err)
			}
		} else {
			err = r.createDockerSecret(ctx, "cfapi-app-registry", ns, appContainerRegistry.Server,
				appContainerRegistry.User, appContainerRegistry.Pass)
			if err != nil {
				return fmt.Errorf("failed to create image pull secret: %w", err)
			}
		}
	}

	return nil
}

func (r *CFAPIReconciler) installGatewayAPI(ctx context.Context) error {
	err := r.installOneGlob(ctx, "./module-data/gateway-api/experimental-install.yaml")
	if err != nil {
		return fmt.Errorf("failed to install the Gateway API: %w", err)
	}

	deploy := &appsv1.Deployment{}
	err = r.k8sClient.Get(context.Background(), client.ObjectKey{
		Namespace: "istio-system",
		Name:      "istiod",
	}, deploy)
	if err != nil {
		return fmt.Errorf("failed to get istiod deployment: %w", err)
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

		if err = r.ssa(ctx, deploy); client.IgnoreAlreadyExists(err) != nil {
			return fmt.Errorf("failed to patch istiod deployment: %w", err)
		}
	}

	err = r.installOneGlob(ctx, "./module-data/envoy-filter/empty-envoy-filter.yaml")
	if err != nil {
		return fmt.Errorf("failed to install evoy filter: %w", err)
	}

	return nil
}

func getStatusFromSample(objectInstance *v1alpha1.CFAPI) v1alpha1.CFAPIStatus {
	return objectInstance.Status
}

// Korifi HELM chart deployment
func (r *CFAPIReconciler) deployKorifi(ctx context.Context, appsDomain, korifiAPIDomain, containerRegistryServer, uaaURL string) error {
	logger := log.FromContext(ctx)

	chart, err := loader.Load("./module-data/korifi-chart")
	if err != nil {
		return fmt.Errorf("failed to load korifi helm chart: %w", err)
	}

	values, err := loadOneYaml("./module-data/korifi/values.yaml")
	if err != nil {
		return fmt.Errorf("failed to load CFAPI values for korifi helm chart: %w", err)
	}

	valuesDynamic := map[string]any{
		"generateInternalCertificates": false,
		"api": map[string]any{
			"apiServer": map[string]any{
				"url": korifiAPIDomain,
			},
			"uaaURL": uaaURL,
		},
		"kpackImageBuilder": map[string]any{
			"builderRepository": containerRegistryServer + "/cfapi/kpack-builder",
		},
		"containerRepositoryPrefix": containerRegistryServer + "/",
		"defaultAppDomainName":      appsDomain,
		"experimental": map[string]any{
			"uaa": map[string]any{
				"enabled": true,
				"url":     uaaURL,
			},
		},
	}

	DeepUpdate(values, valuesDynamic)

	err = applyRelease(chart, "korifi", "korifi", values, logger)

	return err
}

func (r *CFAPIReconciler) retrieveUaaUrl(ctx context.Context) (string, error) {
	logger := log.FromContext(ctx)

	logger.Info("Retrieve UAA url from in-cluster details")

	btpServiceOperatorSecret := corev1.Secret{}
	err := r.k8sClient.Get(context.Background(), client.ObjectKey{
		Namespace: kymaSystemNamespace,
		Name:      btpServiceOperatorSecretName,
	}, &btpServiceOperatorSecret)
	if err != nil {
		return "", fmt.Errorf("failed to get the btp service operator secret: %w", err)
	}

	tokenUrl := string(btpServiceOperatorSecret.Data["tokenurl"])

	logger.Info("Token url extracted from btp service operator secret: " + tokenUrl)

	uaaURL := extractUaaURLFromTokenUrl(tokenUrl)

	logger.Info("UAA url extracted from token url: " + uaaURL)

	return uaaURL, nil
}

func extractUaaURLFromTokenUrl(tokenUrl string) string {
	// input => https://worker1-q3zjpctt.authentication.eu12.hana.ondemand.com
	// output => https://uaa.cf.eu12.hana.ondemand.com

	parts := strings.Split(tokenUrl, ".")
	parts = parts[2:]

	return uaaURLPrefix + strings.Join(parts, ".")
}
