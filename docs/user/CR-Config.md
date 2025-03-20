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


3. ### Istio installed ###

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

4. ### Deploy CF API ###

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

5.  ### Configure CF cli ###

    Set cf cli to point to CF API 
```
cf api https://cfapi.cc6e362.kyma.ondemand.com 
```

6. ### CF Login ###
 
```
cf login --sso
```
