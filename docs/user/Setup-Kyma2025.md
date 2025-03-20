## Enable CF API on Kyma (after September 2024)

0. ### Prerequisites ###
You have a Kyma environment, created after September 2024, which Kyma has CRD of type
authentication.gardener.cloud/OpenIDConnect


1. ### Istio installed ###

Make sure istio kyma module is installed. On a SAP managed Kyma that is the case. 
If Istio is not installed, that could be done so: 
```
	kubectl label namespace cfapi-system istio-injection=enabled --overwrite
	kubectl apply -f https://github.com/kyma-project/istio/releases/latest/download/istio-manager.yaml
	kubectl apply -f https://github.com/kyma-project/istio/releases/latest/download/istio-default-cr.yaml
```

2. ### Deploy CF API ###

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

3.  ### Configure CF cli ###

    Set cf cli to point to CF API 
```
cf api https://cfapi.cc6e362.kyma.ondemand.com 
```

4. ### CF Login ###
 
```
cf login --sso
```
