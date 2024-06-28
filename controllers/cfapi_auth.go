package controllers

import (
	"context"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *CFAPIReconciler) getUserClusterAdmins(ctx context.Context) (error, []rbacv1.Subject) {
	subjects := []rbacv1.Subject{}
	crblist := &rbacv1.ClusterRoleBindingList{}
	err := r.Client.List(ctx, crblist, client.MatchingLabels{"app": "kyma"})
	if err != nil {
		return err, subjects
	}
	for _, crb := range crblist.Items {
		if crb.RoleRef.Name == "cluster-admin" {
			for _, subject := range crb.Subjects {
				if subject.Kind == "User" {
					subjects = append(subjects, subject)
				}
			}
		}
	}
	return nil, subjects
}

func toSubjectList(users []string) []rbacv1.Subject {
	if users == nil {
		return nil
	}
	var subjects = make([]rbacv1.Subject, len(users))
	for i, user := range users {
		subjects[i] = rbacv1.Subject{
			Kind: "User",
			Name: user,
		}
	}
	return subjects
}

func (r *CFAPIReconciler) assignCfAdministrators(ctx context.Context, subjects []rbacv1.Subject, cfNs string) error {
	logger := log.FromContext(ctx)
	var err error
	_subjects := subjects

	if len(subjects) == 0 {
		logger.Info("No CF administrators specified, will set kyma cluster admins as CF administrators")
		err, _subjects = r.getUserClusterAdmins(ctx)
		if err != nil {
			logger.Error(err, "Failed to list users having clusterrole/cluster-admin")
			return nil
		}
		if len(_subjects) == 0 {
			logger.Info("No users with kyma cluster-admin role found, no CF administrators set")
			return nil
		}
	}

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cfapi-admins-binding",
			Namespace: cfNs,
			Annotations: map[string]string{
				"cloudfoundry.org/propagate-cf-role": "true",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "korifi-controllers-admin",
		},
		Subjects: _subjects,
	}

	userNames := make([]string, len(_subjects))
	for i, subject := range _subjects {
		userNames[i] = subject.Name
	}
	logger.Info("Bind role/korifi-controllers-admin to cluser-admin users " + strings.Join(userNames, ","))

	return r.createIfMissing(ctx, rb)
}
