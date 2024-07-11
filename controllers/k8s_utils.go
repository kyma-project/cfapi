package controllers

import (
	"context"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *CFAPIReconciler) crdExists(ctx context.Context, kind string) bool {
	logger := log.FromContext(ctx)

	_ = v1.AddToScheme(r.Scheme)

	crds := &v1.CustomResourceDefinitionList{}
	err := r.Client.List(ctx, crds)

	if err != nil {
		logger.Error(err, "error listing CRDs")
		return false
	}

	for _, i := range crds.Items {
		if i.Spec.Names.Kind == kind {
			return true
		}
	}

	return false
}

func (r *CFAPIReconciler) secretExists(namespace, name string) bool {
	secret := corev1.Secret{}

	err := r.Client.Get(context.Background(), client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &secret)

	return err == nil
}

func (r *CFAPIReconciler) patchDockerSecret(ctx context.Context, name, namespace, server, username, password string) error {
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

	err = r.Client.Patch(context.Background(), &secret, client.MergeFrom(&corev1.Secret{}))

	if err != nil {
		logger.Error(err, "error patching "+name+" secret in ns "+namespace)
		return err
	}

	return nil
}

// ssaStatus patches status using SSA on the passed object.
func (r *CFAPIReconciler) ssaStatus(ctx context.Context, obj client.Object) error {
	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")
	return r.Client.Status().Patch(ctx, obj, client.Apply,
		&client.SubResourcePatchOptions{PatchOptions: client.PatchOptions{FieldManager: fieldOwner}})
}

// ssa patches the object using SSA.
func (r *CFAPIReconciler) ssa(ctx context.Context, obj client.Object) error {
	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")

	return r.Client.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner(fieldOwner))
}

func (r *CFAPIReconciler) createIfMissing(ctx context.Context, object client.Object) error {
	err := r.Client.Create(ctx, object)
	if !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}
