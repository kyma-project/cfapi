#!/bin/bash

set -euo pipefail

export VERSION="$(uuidgen)"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$SCRIPT_DIR/.."
KORIFI_LOCAL_DIR="$ROOT_DIR/korifi-local"
RELEASE_OUTPUT_DIR="$SCRIPT_DIR/release-output"
RELEASE_ARTIFACTS_DIR="$RELEASE_OUTPUT_DIR/korifi-$VERSION"

if [[ -z "${KUBECONFIG:-}" ]]; then
  echo "KUBECONFIG is not set!"
  echo "Please make sure you have targeted your Kyma cluster!"
  exit 1
fi

tmp_dir="$(mktemp -d)"
trap "rm -rf $tmp_dir" EXIT

rm -rf "$RELEASE_OUTPUT_DIR"
mkdir -p "$RELEASE_ARTIFACTS_DIR"

configure_kyma_docker_registry() {
  kubectl apply -f https://github.com/kyma-project/docker-registry/releases/latest/download/dockerregistry-operator.yaml
  kubectl apply -f "$KORIFI_LOCAL_DIR/kyma-docker-registry.yaml"

  kubectl -n kyma-system wait --for=jsonpath='{.status.state}'=Ready dockerregistries/default --timeout=5m

  export REGISTRY_URL=$(kubectl get secret dockerregistry-config-external -o="jsonpath={.data.pushRegAddr}" | base64 -d)
  export REGISTRY_USER=$(kubectl get secret dockerregistry-config-external -o="jsonpath={.data.username}" | base64 -d)
  export REGISTRY_PASSWORD=$(kubectl get secret dockerregistry-config-external -o="jsonpath={.data.password}" | base64 -d)

  while ! curl -o /dev/null "https://$REGISTRY_URL/v2/_catalog" 2>/dev/null; do
    echo Waiting for the docker registry to respond...
    sleep 1
  done

  registry_status_code=""
  while [[ "$registry_status_code" != "200" ]]; do
    echo Waiting for the local docker registry to start...
    registry_status_code=$(curl -L -o /dev/null -w "%{http_code}" --user "$REGISTRY_USER:$REGISTRY_PASSWORD" "https://$REGISTRY_URL/v2/_catalog" 2>/dev/null)
    sleep 1
  done
}

configure_buildkit() {
  # Workarounds https://github.com/moby/buildkit/issues/4764
  kubectl buildkit create --image moby/buildkit:v0.12.5

  kubectl delete secret buildkit &>/dev/null || true

  kubectl create secret docker-registry buildkit --docker-server="$REGISTRY_URL" \
    --docker-username="$REGISTRY_USER" --docker-password="$REGISTRY_PASSWORD"
}

build-korifi() {
  export IMAGE_REPO="$REGISTRY_URL/kyma-project"
  cat "$KORIFI_LOCAL_DIR/korifi-kbld.yml" | envsubst >"$tmp_dir/korifi-kbld.yml"

  yq -i "with(.destinations[]; .tags=[\"latest\", \"$VERSION\"])" "$tmp_dir/korifi-kbld.yml"
  yq -i "with(.sources[]; .docker.buildx.rawOptions += [\"--build-arg\", \"version=$VERSION\"])" "$tmp_dir/korifi-kbld.yml"

  cp "helm/korifi/values.yaml" "$tmp_dir/korifi-values.yaml"

  yq -i e '.systemImagePullSecrets |= ["dockerregistry-config-external"]' "$tmp_dir/korifi-values.yaml"

  kbld -f "$tmp_dir/korifi-values.yaml" -f "$tmp_dir/korifi-kbld.yml" >"$RELEASE_ARTIFACTS_DIR/values.yaml"

}

create_korifi_release() {
  pushd "$HOME/workspace/korifi"
  {
    cp -a helm/korifi/* "$RELEASE_ARTIFACTS_DIR"
    build-korifi
  }
  popd

  pushd "$RELEASE_OUTPUT_DIR"
  {
    tar czf "korifi-$VERSION.tgz" "korifi-$VERSION"
  }
  popd
}

update_cfapi() {
  pushd $ROOT_DIR
  {
    make docker-build REGISTRY=$REGISTRY_URL VERSION=$VERSION
    operator_img="$REGISTRY_URL/kyma-project/cfapi/cfapi-controller:$VERSION"

    docker push "$operator_img"

    make build-manifests
    sed -i "s|image: .*|image: $operator_img|" cfapi-operator.yaml
    kubectl apply -f cfapi-operator.yaml

    kubectl patch deployment -n cfapi-system cfapi-operator -p '{"spec": {"template": {"spec": {"imagePullSecrets": [{"name": "dockerregistry-config-external"}]}}}}'
    kubectl apply -f "$KORIFI_LOCAL_DIR/cf-api.yaml"
    kubectl -n cfapi-system wait --for=jsonpath='{.status.state}'=Ready cfapis/default-cf-api --timeout=10m
  }
  popd

}

main() {
  configure_kyma_docker_registry
  configure_buildkit
  create_korifi_release
  update_cfapi

  echo
  echo "Done"
  echo "To target the CF API run"
  echo "cf login --sso -a " $(kubectl -n cfapi-system get cfapis/default-cf-api -ojsonpath="{.status.url}")
}

main
