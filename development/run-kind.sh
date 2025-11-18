#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$SCRIPT_DIR/.."

# If BUILD_LOCAL_KORIFI is set to true, the script will build Korifi from local
# source code and package it into the operator image that is deployed on the
# kind cluster. Otherwise, the operator will install latest Korifi release.
export BUILD_LOCAL_KORIFI="${BUILD_LOCAL_KORIFI:-false}"

source "$SCRIPT_DIR/tools/common.sh"

export VERSION="$(uuidgen)"

if [[ -n "${KUBECONFIG:-}" ]]; then
  echo "KUBECONFIG is set to $KUBECONFIG!"
  echo "Please unset it to prevent contaminating your current cluster's config file."
  exit 1
fi

tmp_dir="$(mktemp -d)"
trap "rm -rf $tmp_dir" EXIT

apply_cfapi() {
  cat "$SCRIPT_DIR/assets/cf-api.yaml" | envsubst | kubectl apply -f -
  kubectl -n cfapi-system wait --for=jsonpath='{.status.state}'=Ready cfapis/default-cf-api --timeout=10m

  configure_gateway_service cfapi-system "$KORIFI_GW_SERVICE" "$KORIFI_GW_TLS_PORT"
}

create_default_admins() {
  kubectl apply -f "$SCRIPT_DIR/assets/admin-users.yaml"
}

create_btp_operator_secret() {
  echo "************************************************"
  echo " Creating the BTP Operator Module Secret"
  echo "************************************************"

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
}

main() {
  login

  "$ROOT_DIR/development/prepare-kind.sh" cfapi
  source "$ROOT_DIR/development/assets/secrets/env/env.sh"
  create_btp_operator_secret

  create_default_admins

  apply_cfapi

  cfapi_url=$(kubectl -n cfapi-system get cfapis.operator.kyma-project.io default-cf-api -ojsonpath='{.status.url}')
  echo "CF API: $cfapi_url"
  echo
  echo "To login, run:"
  echo "cf login --sso --skip-ssl-validation -a $cfapi_url"
}

main
