#!/bin/bash

kubectl delete --ignore-not-found --namespace cfapi-system cfapis.operator.kyma-project.io default-cf-api
kubectl delete --ignore-not-found namespace cf
helm delete --ignore-not-found btp-service-broker -n cfapi-system --wait
helm delete --ignore-not-found cfapi-config -n korifi --wait
helm delete --ignore-not-found korifi -n korifi --wait
helm delete --ignore-not-found korifi-prerequisites -n korifi --wait
helm delete --ignore-not-found contour -n cfapi-system --wait
kubectl delete --ignore-not-found namespace korifi
kubectl delete --ignore-not-found -f $HOME/workspace/cfapi/module-data/issuers/issuers.yaml
kubectl delete --ignore-not-found -f $HOME/workspace/cfapi/module-data/vendor/gateway-api
kubectl delete --ignore-not-found -f $HOME/workspace/cfapi/module-data/vendor/kpack
