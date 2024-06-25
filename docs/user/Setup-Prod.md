## How to enable CF API on a productive BTP landscape


1. ### Kyma environment ###

    You need a Kyma environment which is configured with UAA as an OIDC provider, with following parameters
``` yaml
{
  "administrators": [
    "sap.ids:myfirstname.mysecondname@sap.com"
  ],
  "autoScalerMax": 3,
  "autoScalerMin": 3,
  "oidc": {
    "clientID": "cf",
    "groupsClaim": "",
    "issuerURL": "https://uaa.cf.eu10.hana.ondemand.com/oauth/token",
    "signingAlgs": [
      "RS256"
    ],
    "usernameClaim": "user_name",
    "usernamePrefix": "sap.ids:"
  }
}
```

2. ### Kyma Access ###

    Use that script to generate a stable kubeconfig: <br>
    https://github.tools.sap/unified-runtime/trinity/blob/main/scripts/tools/generate-kyma-kubeconfig.sh
    
    Note: that step requires an UAA client (uaac), which requires Ruby runtime

3. ### Registry secret ###

    Create a system docker registry with name **cfapi-system-registry**, namespace cfapi-system. That registry contains the system images for the kyma module.     Currently that is manual step, that the cfapi-kyma-module dev team has to provide credentials to SAP artifactory
    The easiest way is to execute:
``` bash
export DOCKER_REGISTRY=trinity.common.repositories.cloud.sap
export DOCKER_REGISTRY_USER=korifi-dev
export DOCKER_REGISTRY_PASS=*******************

kubectl create namespace cfapi-system --dry-run=client -o yaml | kubectl apply -f -
kubectl create -n cfapi-system secret docker-registry cfapi-system-registry --docker-server=${DOCKER_REGISTRY} --docker-username=${DOCKER_REGISTRY_USER} --docker-password=${DOCKER_REGISTRY_PASS}
```

4. ### Istio installed ###

    If Istio Kyma module is not installed, you can do it with:

*make* from this repository
```
make install-istio
```
Or directly with kubectl
```
	kubectl label namespace cfapi-system istio-injection=enabled --overwrite
	kubectl apply -f https://github.com/kyma-project/istio/releases/latest/download/istio-manager.yaml
	kubectl apply -f https://github.com/kyma-project/istio/releases/latest/download/istio-default-cr.yaml
```

5. ### Deploy CF API ###

    Deploy the resources from a particular release version to kyma
```
kubectl apply -f cfapi-crd.yaml
kubectl apply -f cfapi-manager.yaml
kubectl apply -f cfapi-default-cr.yaml
```

  Wait for a Ready state of the CFAPI resource and read the CF URL 
```
kubectl get -n cfapi-system cfapi
NAME             STATE   URL
default-cf-api   Ready   https://cfapi.cc6e362.kyma.ondemand.com
```

7.  ### Configure CF cli ###

    Set cf cli to point to CF API 
```
cf api https://cfapi.cc6e362.kyma.ondemand.com 
```

8. ### CF Login ###
 
```
cf login --sso
```
