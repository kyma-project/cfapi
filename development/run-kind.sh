#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$SCRIPT_DIR/.."
KORIFI_DIR="$ROOT_DIR/../korifi"
CLOUD_PROVIDER_KIND_DIR="$ROOT_DIR/../cloud-provider-kind"

export VERSION="$(uuidgen)"

if [[ -n "${KUBECONFIG:-}" ]]; then
  echo "KUBECONFIG is set to $KUBECONFIG!"
  echo "Please unset it to prevent contaminating your current cluster's config file."
  exit 1
fi

SECRET_DIR="$SCRIPT_DIR/assets/secrets"
SSL_DIR="$SECRET_DIR/ssl"
mkdir -p "$SSL_DIR"

rm -rf "$ROOT_DIR/release"
RELEASE_OUTPUT_DIR="$ROOT_DIR/release/$VERSION"
KORIFI_RELEASE_ARTIFACTS_DIR="$RELEASE_OUTPUT_DIR/korifi-$VERSION"

rm -rf "$RELEASE_OUTPUT_DIR"
mkdir -p "$KORIFI_RELEASE_ARTIFACTS_DIR"

KYMA_TLS_PORT=8443
KYMA_GW_TLS_PORT=31443
KORIFI_GW_TLS_PORT=32443

source "$SCRIPT_DIR/tools/common.sh"
export UAA_URL

tmp_dir="$(mktemp -d)"
trap "rm -rf $tmp_dir" EXIT

download_uaa_ca_pem() {
  openssl s_client -showcerts -connect ${UAA_URL#https://}:443 </dev/null >"$SSL_DIR/ca.pem"
}

ensure_kind_cluster() {
  local cluster="$1"
  if ! kind get clusters | grep -q "${cluster}"; then
    download_uaa_ca_pem
    cat <<EOF | kind create cluster --name "${cluster}" --image kindest/node:v1.32.5 --wait 5m --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry]
    [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
      [plugins."io.containerd.grpc.v1.cri".registry.mirrors."dockerregistry.kyma-system.svc.cluster.local:5000"]
        endpoint = ["http://127.0.0.1:32137"]
nodes:
- role: control-plane
  extraMounts:
  - containerPath: /var/run/docker.sock
    hostPath: /var/run/docker.sock
  - containerPath: /ssl
    hostPath: $SSL_DIR
    readOnly: true
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  - |
    kind: ClusterConfiguration
    apiServer:
      extraVolumes:
        - name: docker-socket
          hostPath: /var/run/docker.sock
          mountPath: /var/run/docker.sock
        - name: ssl-certs
          hostPath: /ssl
          mountPath: /etc/uaa-ssl
      extraArgs:
        oidc-issuer-url: ${UAA_URL}/oauth/token
        oidc-client-id: cloud_controller
        oidc-ca-file: /etc/uaa-ssl/ca.pem
        oidc-username-claim: user_name
        oidc-username-prefix: "$OIDC_PREFIX:"
        oidc-signing-algs: "RS256"
  extraPortMappings:
  - containerPort: $KORIFI_GW_TLS_PORT
    hostPort: 443
    protocol: TCP
  - containerPort: $KYMA_GW_TLS_PORT
    hostPort: $KYMA_TLS_PORT
    protocol: TCP
EOF

  fi
  kind export kubeconfig --name "${cluster}"
}

install_metrics_server() {
  echo "************************************************"
  echo " Installing Metrics Server Insecure TLS options"
  echo "************************************************"

  local dep_dir vendor_dir
  dep_dir="${KORIFI_DIR}/tests/dependencies"
  vendor_dir="${KORIFI_DIR}/tests/vendor"

  trap "rm $dep_dir/insecure-metrics-server/components.yaml" EXIT
  cp "$vendor_dir/metrics-server-local/components.yaml" "$dep_dir/insecure-metrics-server/components.yaml"
  kubectl apply -k "$dep_dir/insecure-metrics-server"
}

install_istio() {
  echo "************************************************"
  echo " Installing the Istio Module "
  echo "************************************************"
  kubectl apply -f https://github.com/kyma-project/istio/releases/latest/download/istio-manager-experimental.yaml
  kubectl apply -f https://github.com/kyma-project/istio/releases/latest/download/istio-default-cr.yaml

  kubectl wait --for=jsonpath='.status.state'=Ready -n kyma-system istios default
  configure_gateway_service istio-system istio-ingressgateway "$KYMA_GW_TLS_PORT"

  echo "************************************************"
  echo " Creating the Default Istio Gateway "
  echo "************************************************"
  kubectl apply -f "$SCRIPT_DIR/assets/kyma-gateway.yaml"
}

install_docker_registry() {
  echo "************************************************"
  echo " Installing the Docker Registry Module "
  echo "************************************************"
  kubectl apply -f https://github.com/kyma-project/docker-registry/releases/latest/download/dockerregistry-operator.yaml
  kubectl apply -f "$SCRIPT_DIR/assets/kyma-docker-registry.yaml"

  kubectl -n kyma-system wait --for=jsonpath='{.status.state}'=Ready dockerregistries/default --timeout=5m

  export REGISTRY_URL="$(kubectl get secret dockerregistry-config-external -o="jsonpath={.data.pushRegAddr}" | base64 -d):$KYMA_TLS_PORT"
  export IN_CLUSTER_REGISTRY_URL=$(kubectl get secret dockerregistry-config -o="jsonpath={.data.pushRegAddr}" | base64 -d)
  export REGISTRY_USER=$(kubectl get secret dockerregistry-config-external -o="jsonpath={.data.username}" | base64 -d)
  export REGISTRY_PASSWORD=$(kubectl get secret dockerregistry-config-external -o="jsonpath={.data.password}" | base64 -d)

  while ! curl -k -o /dev/null "https://$REGISTRY_URL/v2/_catalog" 2>/dev/null; do
    echo Waiting for the docker registry to respond...
    sleep 1
  done

  registry_status_code=""
  while [[ "$registry_status_code" != "200" ]]; do
    echo Waiting for the local docker registry to start...
    registry_status_code=$(curl -k -L -o /dev/null -w "%{http_code}" --user "$REGISTRY_USER:$REGISTRY_PASSWORD" "https://$REGISTRY_URL/v2/_catalog" 2>/dev/null)
    sleep 1
  done

  docker login "$REGISTRY_URL" --username "$REGISTRY_USER" --password $REGISTRY_PASSWORD
}

install_gardener_cert_manager() {
  echo ">>> Installing Gateway API"
  kubectl apply -f "$KORIFI_DIR/tests/vendor/gateway-api"

  echo ">>> Installing Vertical Pod Autoscaler"
  kubectl apply -f https://raw.githubusercontent.com/kubernetes/autoscaler/vpa-release-1.0/vertical-pod-autoscaler/deploy/vpa-v1-crd-gen.yaml
  kubectl apply -f https://raw.githubusercontent.com/kubernetes/autoscaler/vpa-release-1.0/vertical-pod-autoscaler/deploy/vpa-rbac.yaml

  echo ">>> Installing Gardener cert-manager"
  helm repo add gardener-charts https://gardener-community.github.io/gardener-charts

  # `external-dns-management.enabled=true` - installs the DNSEntry custom resource, the gardener cert manager requires it
  # `controller.enabled=true` - not sure whether we want a controller for DNS manadement, provided we are running on kind
  helm upgrade --install \
    external-dns-management gardener-charts/external-dns-management \
    --namespace kyma-system \
    --set external-dns-management.enabled=true \
    --set controller.enabled=true \
    --wait

  # configuration.issuerNamespace=kyma-system - cert manager will treat issuers in that namespace as issuers from the `default` cluster, see https://github.com/gardener/cert-management#using-the-cert-controller-manager
  helm upgrade --install \
    cert-management gardener-charts/cert-management \
    --namespace kyma-system \
    --set configuration.issuerNamespace=kyma-system \
    --set configuration.defaultIssuer=default-issuer \
    --wait

  kubectl apply -f "$SCRIPT_DIR/assets/cert-issuer.yaml"
}

create_namespaces() {
  kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: kyma-system

---
apiVersion: v1
kind: Namespace
metadata:
  name: korifi
EOF
}

build_korifi() {
  echo "Building korifi values file..."

  make generate manifests

  kbld_file="scripts/assets/korifi-kbld.yml"

  values_file=""$KORIFI_RELEASE_ARTIFACTS_DIR"/values.yaml"

  CHART_VERSION="0.0.0-$VERSION" yq -i 'with(.; .version=env(CHART_VERSION))' "$KORIFI_RELEASE_ARTIFACTS_DIR/Chart.yaml"
  yq "with(.sources[]; .docker.buildx.rawOptions += [\"--build-arg\", \"version=$VERSION\"])" $kbld_file |
    kbld \
      --images-annotation=false \
      -f "helm/korifi/values.yaml" \
      -f - >"$values_file"

  awk '/image:/ {print $2}' "$values_file" | while read -r img; do
    push_img="$REGISTRY_URL/cloudfoundry/$img"

    docker tag "$img" "$push_img"
    docker push "$push_img"
  done

  sed -i "s|  image: |  image: $IN_CLUSTER_REGISTRY_URL/cloudfoundry/|" "$values_file"
  yq -i e '.systemImagePullSecrets |= ["dockerregistry-config"]' "$values_file"
}

build_korifi_release_chart() {
  pushd "$KORIFI_DIR"
  {
    cp -a helm/korifi/* "$KORIFI_RELEASE_ARTIFACTS_DIR"
    build_korifi
  }
  popd

  pushd "$RELEASE_OUTPUT_DIR"
  {
    tar czf "korifi-$VERSION.tgz" "korifi-$VERSION"
  }
  popd
}

install_cfapi() {
  kubectl -n korifi delete secret cfapi-registry-secret --ignore-not-found=true
  kubectl -n korifi create secret generic cfapi-registry-secret \
    --from-literal=server=$IN_CLUSTER_REGISTRY_URL \
    --from-literal=username=$REGISTRY_USER \
    --from-literal=password=$REGISTRY_PASSWORD

  pushd $ROOT_DIR
  {
    make release REGISTRY=$REGISTRY_URL VERSION=$VERSION

    broker_incluster_image="$IN_CLUSTER_REGISTRY_URL/kyma-project/cfapi/btp-service-broker:$VERSION"
    broker_incluster_image=$broker_incluster_image yq -i 'with(.broker; .image=env(broker_incluster_image))' release/$VERSION/btp-service-broker/helm/values.yaml

    cf_api_operator_image="$REGISTRY_URL/kyma-project/cfapi/cfapi-controller:$VERSION-kind"
    docker build \
      --build-arg VERSION="$VERSION" \
      --build-arg REGISTRY="$REGISTRY_URL" \
      --build-arg IMG="kyma-project/cfapi/cfapi-controller" \
      -t "$cf_api_operator_image" \
      -f "$SCRIPT_DIR/assets/Dockerfile" .

    docker push "$cf_api_operator_image"

    cf_api_operator_incluster_image="$IN_CLUSTER_REGISTRY_URL/kyma-project/cfapi/cfapi-controller:$VERSION-kind"
    sed -i "s|image: .*|image: $cf_api_operator_incluster_image|" release/$VERSION/cfapi/cfapi-operator.yaml
    kubectl apply -f release/$VERSION/cfapi/cfapi-operator.yaml
    kubectl patch deployment -n cfapi-system cfapi-operator -p '{"spec": {"template": {"spec": {"imagePullSecrets": [{"name": "dockerregistry-config"}]}}}}'

    cat "$SCRIPT_DIR/assets/cf-api.yaml" | envsubst | kubectl apply -f -

    kubectl -n cfapi-system wait --for=jsonpath='{.status.state}'=Ready cfapis/default-cf-api --timeout=10m
  }
  popd

  configure_gateway_service korifi-gateway korifi-istio "$KORIFI_GW_TLS_PORT"
}

configure_gateway_service() {
  ns="$1"
  service="$2"
  nodePort="$3"

  kubectl patch service -n "$ns" "$service" --patch-file <(
    cat <<EOF
spec:
  ports:
  - name: https
    nodePort: $nodePort
    port: 443
    protocol: TCP
    targetPort: 8443
EOF
  )
}

create_default_admins() {
  kubectl apply -f "$SCRIPT_DIR/assets/admin-users.yaml"
}

install_load_balancer() {
  echo "************************************************"
  echo " Installing Load Balancer "
  echo "************************************************"

  pushd "$CLOUD_PROVIDER_KIND_DIR"
  {
    docker build . -t $REGISTRY_URL/cloud-provider-kind:$VERSION
    docker push $REGISTRY_URL/cloud-provider-kind:$VERSION
  }
  popd

  cat $SCRIPT_DIR/assets/cloud-provider-kind.yaml |
    CLOUD_PROVIDER_KIND_IMAGE="$IN_CLUSTER_REGISTRY_URL/cloud-provider-kind:$VERSION" envsubst |
    kubectl apply -f -
}

install_btp_operator() {
  plan_id="$(btp --format=json list services/plans | jq -r '.[] | select(.name=="service-operator-access") | .id')"
  instance_id="$(btp --format=json list services/instances | jq -r ".[] | select(.service_plan_id==\"$plan_id\") | .id")"
  credentials="$(btp --format=json list services/bindings | jq -r ".[] | select(.service_instance_id==\"$instance_id\") | .credentials")"

  kubectl --namespace kyma-system delete secret sap-btp-manager --ignore-not-found
  kubectl --namespace kyma-system create secret generic sap-btp-manager \
    --from-literal=sm_url="$(jq --raw-output '.sm_url' <<<"$credentials")" \
    --from-literal=tokenurl="$(jq --raw-output '.url' <<<"$credentials")" \
    --from-literal=clientid="$(jq --raw-output '.clientid' <<<"$credentials")" \
    --from-literal=clientsecret="$(jq --raw-output '.clientsecret' <<<"$credentials")" \
    --from-literal=cluster_id="3690e037-73fb-4d2d-9df0-5a8cb4382985"
  kubectl --namespace kyma-system label secret sap-btp-manager "app.kubernetes.io/managed-by=kcp-kyma-environment-broker"

  echo "************************************************"
  echo " Installing the BTP Operator Module "
  echo "************************************************"
  kubectl apply -f https://github.com/kyma-project/btp-manager/releases/latest/download/btp-manager.yaml
  kubectl apply -f https://github.com/kyma-project/btp-manager/releases/latest/download/btp-operator.yaml
}

main() {
  login

  ensure_kind_cluster cfapi
  create_default_admins

  create_namespaces
  install_gardener_cert_manager
  install_istio
  install_docker_registry
  install_metrics_server
  install_load_balancer
  install_btp_operator

  build_korifi_release_chart
  install_cfapi

  cfapi_url=$(kubectl -n cfapi-system get cfapis.operator.kyma-project.io default-cf-api -ojsonpath='{.status.url}')
  echo "CF API: $cfapi_url"
  echo
  echo "To login, run:"
  echo "cf login --sso --skip-ssl-validation -a $cfapi_url"
}

main
