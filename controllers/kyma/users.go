package kyma

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Users struct {
	k8sClient client.Client
}

func NewUsers(k8sClient client.Client) *Users {
	return &Users{
		k8sClient: k8sClient,
	}
}

func (u *Users) GetClusterAdmins(ctx context.Context) ([]rbacv1.Subject, error) {
	subjects := []rbacv1.Subject{}

	clusterRoleBindings := &rbacv1.ClusterRoleBindingList{}
	err := u.k8sClient.List(ctx, clusterRoleBindings)
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster role bindings: %w", err)
	}

	for _, crb := range clusterRoleBindings.Items {
		if crb.RoleRef.Name != "cluster-admin" {
			continue
		}

		for _, subject := range crb.Subjects {
			if subject.Kind != "User" {
				continue
			}
			subjects = append(subjects, subject)
		}
	}
	return subjects, nil
}
