[![REUSE status](https://api.reuse.software/badge/github.com/kyma-project/cfapi)](https://api.reuse.software/info/github.com/kyma-project/cfapi)

# CFAPI Kyma Module

## Overview
CF API Kyma Module is providing a CF API to run on top of Kyma, using the open source [Korifi](https://github.com/cloudfoundry/korifi) project
Once installed, one could use the cf cli to connect and deploy workloads. 

## Custom Resource (CR) Specification
| Property | Optional | Default | Description |
|-----|-----|-----|-----|
| RootNamespace | Optional | cf | Root namespace for CF resources |
| AppImagePullSecret | Optional | By default a localregistry will be deployed and used for application images | Dockerregistry secret pointing to a custom docker registry |
| UAA | Optional | "https://uaa.cf.eu10.hana.ondemand.com" |  UAA URL to be used for authentication |
| CFAdmins | Optional | By default Kyma cluster-admins will become CF admins | List of users, which will become CF administrators.A user is expected in format sap.ids:\<sap email\> example sap.ids:samir.zeort@sap.com  |


## Prerequisites
* A Kyma cluster
* Istio Kyma module installed
* [Maybe] Docker registry kyma module installed

## Installation
1. ### Setup a Kyma environment ###

2. ### Istio installed ###

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

3. ### [Maybe] deploy docker registry module
In case you want to use an existing docker registry, you do not need to install that.
In all other cases, see https://github.com/kyma-project/docker-registry/blob/main/docs/user/README.md
```
kubectl apply -f https://github.com/kyma-project/docker-registry/releases/latest/download/dockerregistry-operator.yaml
```

4. ### Deploy CF API ###

Deploy the resources from a particular release /in the example below 0.0.9/ version to kyma
```
kubectl create namespace cfapi-system
kubectl apply -f https://github.com/kyma-project/cfapi/releases/download/0.0.9/cfapi-manager.yaml
kubectl apply -f https://github.com/kyma-project/cfapi/releases/download/0.0.9/cfapi-default-cr.yaml
```

  Wait for a Ready state of the CFAPI resource and read the CF URL 
```
kubectl get -n cfapi-system cfapi
NAME             STATE   URL
default-cf-api   Ready   https://cfapi.cc6e362.kyma.ondemand.com
```

5.  ### CF login ###

    Set cf cli to point to CF API 
```
cf login --sso -a https://cfapi.cc6e362.kyma.ondemand.com 
```

   
## Usage

Use CF cli to deploy applications as on a normal CF. The buildpacks used are native community buildpacks. 

## Development

## Contributing
See the [Contributing Rules](CONTRIBUTING.md).

## Code of Conduct
See the [Code of Conduct](CODE_OF_CONDUCT.md) document.

## Licensing

See the [license](./LICENSE) file.
