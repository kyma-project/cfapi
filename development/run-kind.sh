#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$SCRIPT_DIR/.."
KORIFI_DIR="$ROOT_DIR/../korifi"
CLOUD_PROVIDER_KIND_DIR="$ROOT_DIR/../cloud-provider-kind"

source "$SCRIPT_DIR/tools/common.sh"

export VERSION="$(uuidgen)"

if [[ -n "${KUBECONFIG:-}" ]]; then
  echo "KUBECONFIG is set to $KUBECONFIG!"
  echo "Please unset it to prevent contaminating your current cluster's config file."
  exit 1
fi

rm -rf "$ROOT_DIR/release"
RELEASE_OUTPUT_DIR="$ROOT_DIR/release/$VERSION"
KORIFI_RELEASE_ARTIFACTS_DIR="$RELEASE_OUTPUT_DIR/korifi-$VERSION"

rm -rf "$RELEASE_OUTPUT_DIR"
mkdir -p "$KORIFI_RELEASE_ARTIFACTS_DIR"

tmp_dir="$(mktemp -d)"
trap "rm -rf $tmp_dir" EXIT

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

create_default_admins() {
  kubectl apply -f "$SCRIPT_DIR/assets/admin-users.yaml"
}

main() {
  login

  "$ROOT_DIR/development/prepare-kind.sh" cfapi
  source "$ROOT_DIR/development/assets/secrets/env/env.sh"

  create_default_admins

  build_korifi_release_chart
  install_cfapi

  cfapi_url=$(kubectl -n cfapi-system get cfapis.operator.kyma-project.io default-cf-api -ojsonpath='{.status.url}')
  echo "CF API: $cfapi_url"
  echo
  echo "To login, run:"
  echo "cf login --sso --skip-ssl-validation -a $cfapi_url"
}

main
