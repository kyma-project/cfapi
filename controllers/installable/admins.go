package installable

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/BooleanCat/go-functional/v2/it"
	"github.com/kyma-project/cfapi/api/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Admins struct {
	k8sClient client.Client
}

func NewAdmins(k8sClient client.Client) *Admins {
	return &Admins{
		k8sClient: k8sClient,
	}
}

func (a *Admins) Install(ctx context.Context, config v1alpha1.InstallationConfig, eventRecorder EventRecorder) (Result, error) {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cfapi-admins",
			Namespace: config.RootNamespace,
		},
	}

	_, err := controllerutil.CreateOrPatch(ctx, a.k8sClient, rb, func() error {
		if rb.Annotations == nil {
			rb.Annotations = map[string]string{}
		}
		rb.Annotations["cloudfoundry.org/propagate-cf-role"] = "true"
		rb.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "korifi-controllers-admin",
		}
		rb.Subjects = slices.Collect(it.Map(slices.Values(config.CFAdmins), toSubject(config)))

		return nil
	})
	if err != nil {
		return Result{}, fmt.Errorf("failed to create/patch admin role binding: %w", err)
	}

	return Result{State: ResultStateSuccess}, nil
}

func toSubject(config v1alpha1.InstallationConfig) func(string) rbacv1.Subject {
	return func(admin string) rbacv1.Subject {
		subjectName := admin
		if !strings.HasPrefix(admin, "sap.ids:") {
			subjectName = "sap.ids:" + subjectName
		}

		return rbacv1.Subject{
			Kind:      "User",
			Name:      subjectName,
			Namespace: config.RootNamespace,
		}
	}
}
