package controllers

import (
	"bytes"
	"context"
	"text/template"

	"github.tools.sap/unified-runtime/cfapi-kyma-module/api/v1alpha1"
	"helm.sh/helm/v3/pkg/chart/loader"
	corev1 "k8s.io/api/core/v1"
	errors2 "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *CFAPIReconciler) installTwuni(ctx context.Context, cfAPI *v1alpha1.CFAPI, cfDomain, twuniDomain string) error {
	logger := log.FromContext(ctx)

	if cfAPI.Spec.AppImagePullSecret != "" {
		logger.Info("App Container Img Reg Secret is set, skipping twuni installation")
		return nil
	}

	// create twuni certificate
	logger.Info("Start installing twuni ...")
	err := r.createTwuniCertificate(ctx, cfDomain, twuniDomain)
	if err != nil {
		logger.Error(err, "error creating twuni certificate")
		return err
	}

	// deploy twuni helm
	err = r.deployTwuniHelm(ctx)
	if err != nil {
		logger.Error(err, "error deploying twuni helm")
		return err
	}

	// create reference grant
	err = r.createTwuniReferenceGrant(ctx)
	if err != nil {
		logger.Error(err, "error creating twuni reference grant")
		return err
	}

	// create tlsroute
	err = r.createTwuniTLSRoute(ctx, twuniDomain)
	if err != nil {
		logger.Error(err, "error creating twuni tls route")
		return err
	}

	logger.Info("Finished installing twuni ...")

	return nil
}

func (r *CFAPIReconciler) createTwuniTLSRoute(ctx context.Context, twuniDomain string) error {
	logger := log.FromContext(ctx)

	vals := struct {
		TwuniDomain string
	}{
		TwuniDomain: twuniDomain,
	}

	t1 := template.New("twuniTLSRoute")

	t2, err := t1.ParseFiles("./module-data/twuni-tlsroute/tlsroute.tmpl")

	if err != nil {
		logger.Error(err, "error during parsing of twuni tls route template")
		return err
	}

	buf := &bytes.Buffer{}

	err = t2.ExecuteTemplate(buf, "twuniTLSRoute", vals)

	if err != nil {
		logger.Error(err, "error during execution of twuni tls route template")
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

	return nil
}

func (r *CFAPIReconciler) createTwuniReferenceGrant(ctx context.Context) error {
	logger := log.FromContext(ctx)

	err := r.installOneGlob(ctx, "./module-data/twuni-referencegrant/referencegrant.yaml")
	if err != nil {
		logger.Error(err, "error installing twuni reference grant resources")
		return err
	}

	return nil
}

func (r *CFAPIReconciler) createTwuniDNSEntry(ctx context.Context, cfAPI *v1alpha1.CFAPI, twuniDomain string) error {
	logger := log.FromContext(ctx)

	if cfAPI.Spec.AppImagePullSecret != "" {
		logger.Info("App Container Img Reg Secret is set, skipping twuni installation")
		return nil
	}

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
		TwuniDomain string
		IngressHost string
	}{
		TwuniDomain: twuniDomain,
		IngressHost: hostname,
	}

	t1 := template.New("twuniDNSEntry")

	t2, err := t1.ParseFiles("./module-data/twuni-dns-entry/dnsentry.tmpl")
	if err != nil {
		logger.Error(err, "error during parsing of twuni dns entries template")
		return err
	}

	buf := &bytes.Buffer{}

	err = t2.ExecuteTemplate(buf, "twuniDNSEntry", vals)
	if err != nil {
		logger.Error(err, "error during execution of twuni dns entries template")
		return err
	}

	s := buf.String()

	resourceObjs, err := parseManifestStringToObjects(s)

	if err != nil {
		logger.Error(err, "error during parsing of twuni dns entries")
		return nil
	}

	for _, obj := range resourceObjs.Items {
		if err = r.ssa(ctx, obj); err != nil && !errors2.IsAlreadyExists(err) {
			logger.Error(err, "error during installation of twuni dns entries")
			return err
		}
	}

	return nil
}

func (r *CFAPIReconciler) deployTwuniHelm(ctx context.Context) error {
	logger := log.FromContext(ctx)

	filename, err := findOneGlob("./module-data/twuni-helm/*.tar.gz")
	if err != nil {
		return err
	}

	chart, err := loader.Load(filename)
	if err != nil {
		logger.Error(err, "error during loading twuni helm chart")
		return err
	}

	inputValues := map[string]interface{}{
		"persistence": map[string]interface{}{
			"enabled":       true,
			"deleteEnabled": true,
		},
		"service": map[string]interface{}{
			"port": 30050,
		},
		"secrets": map[string]interface{}{
			"htpasswd": "user:$2y$05$MKUm/h9dwwWoCOht5enn3uyih.awSPDILY.kovYyT3J8KSw5lmwIe",
		},
		"tlsSecretName": "docker-registry-ingress-cert",
	}

	err = applyRelease(chart, "cfapi-system", "localregistry", inputValues, logger)

	return err
}

func (r *CFAPIReconciler) createTwuniCertificate(ctx context.Context, cfDomain, twuniDomain string) error {
	logger := log.FromContext(ctx)

	vals := struct {
		CFDomain    string
		TwuniDomain string
	}{
		CFDomain:    cfDomain,
		TwuniDomain: twuniDomain,
	}

	t1 := template.New("twuniCert")

	t2, err := t1.ParseFiles("./module-data/twuni-certificate/certificate.tmpl")

	if err != nil {
		logger.Error(err, "error during parsing of twuni certificate template")
		return err
	}

	buf := &bytes.Buffer{}

	err = t2.ExecuteTemplate(buf, "twuniCert", vals)

	if err != nil {
		logger.Error(err, "error during execution of twuni certificate template")
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
	err = r.waitForSecret("cfapi-system", "docker-registry-ingress-cert")
	if err != nil {
		logger.Error(err, "error waiting for secret docker-registry-ingress-cert")
		return err
	}

	return nil
}
