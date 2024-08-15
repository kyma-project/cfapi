#!/bin/bash

set -eo pipefail 
set -o xtrace

SCRIPTDIR=$(dirname "$0")

BTP_USER=$1
KUBECONFIG_INPUT=${2:-kubeconfig.yaml}

cp $KUBECONFIG_INPUT kubeconfig-user.yaml
export KUBECONFIG=kubeconfig-user.yaml

if [[ -z "${UAA_URL}" ]]; then
 echo "Env UAA_URL not set"
 exit 1
fi

uaac target $UAA_URL
uaac token sso get cf --secret ""
uaac me

KUBECONFIG_USER="$OIDC_PREFIX:$BTP_USER"
KUBECONFIG_TOKEN=$(yq ".[\"$UAA_URL\"].contexts[\"$BTP_USER\"].access_token" ~/.uaac.yml)

yq -i ".users |= [{\"name\":\"$KUBECONFIG_USER\", \"user\": {\"token\":\"$KUBECONFIG_TOKEN\"}}]" $KUBECONFIG
yq -i ".contexts[0].context.user |= \"$KUBECONFIG_USER\"" $KUBECONFIG

kubectl apply -f $SCRIPTDIR/serviceaccount.yaml
kubectl wait --for=jsonpath='{.data.token}' secret/admin-serviceaccount
SA_TOKEN=$(kubectl get secret admin-serviceaccount -o=go-template='{{.data.token | base64decode}}')

cp $KUBECONFIG kubeconfig-sa.yaml
yq -i ".users |= [{\"name\":\"admin-serviceaccount\", \"user\": {\"token\":\"$SA_TOKEN\"}}]" kubeconfig-sa.yaml
yq -i ".contexts[0].context.user |= \"admin-serviceaccount\"" kubeconfig-sa.yaml

