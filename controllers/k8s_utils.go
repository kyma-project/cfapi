package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *CFAPIReconciler) crdExists(ctx context.Context, kind string) (bool, error) {
	err := v1.AddToScheme(r.scheme)
	if err != nil {
		return false, err
	}

	crds := &v1.CustomResourceDefinitionList{}
	err = r.k8sClient.List(ctx, crds)
	if err != nil {
		return false, fmt.Errorf("failed to list CRDs: %w", err)
	}

	for _, i := range crds.Items {
		if i.Spec.Names.Kind == kind {
			return true, nil
		}
	}

	return false, nil
}

func (r *CFAPIReconciler) secretExists(namespace, name string) bool {
	secret := corev1.Secret{}

	err := r.k8sClient.Get(context.Background(), client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &secret)

	return err == nil
}

func (r *CFAPIReconciler) patchDockerSecret(ctx context.Context, name, namespace, server, username, password string) error {
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

	err = r.k8sClient.Patch(context.Background(), &secret, client.MergeFrom(&corev1.Secret{}))
	if err != nil {
		return fmt.Errorf("failed to patch secret: %w", err)
	}

	return nil
}

// ssaStatus patches status using SSA on the passed object.
func (r *CFAPIReconciler) ssaStatus(ctx context.Context, obj client.Object) error {
	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")
	return r.k8sClient.Status().Patch(ctx, obj, client.Apply,
		&client.SubResourcePatchOptions{PatchOptions: client.PatchOptions{FieldManager: fieldOwner}})
}

// ssa patches the object using SSA.
func (r *CFAPIReconciler) ssa(ctx context.Context, obj client.Object) error {
	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")

	return r.k8sClient.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner(fieldOwner))
}

func (r *CFAPIReconciler) createIfMissing(ctx context.Context, object client.Object) error {
	return client.IgnoreAlreadyExists(r.k8sClient.Create(ctx, object))
}
