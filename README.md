[![REUSE status](https://api.reuse.software/badge/github.com/kyma-project/cfapi)](https://api.reuse.software/info/github.com/kyma-project/cfapi)

# CFAPI Kyma Module

## Overview
CF API Kyma Module is providing a CF API to run on top of Kyma, using the open source [Korifi](https://github.com/cloudfoundry/korifi) project
Once installed, one could use the cf cli to connect and deploy workloads. 

 ![CF on Kyma Overview](/docs/user/cfapi-kyma.drawio.png)

## Custom Resource (CR) Specification
| Property | Optional | Default | Description |
|-----|-----|-----|-----|
| RootNamespace | Optional | cf | Root namespace for CF resources |
| AppImagePullSecret | Optional | By default a local DockerRegistry will be deployed and used | Dockerregistry secret pointing to a docker registry for application images|
| UAA | Optional | In case of a SAP managed Kyma, the UAA will be derived from the BTP service operator config |  UAA URL to be used for authentication |
| CFAdmins | Optional | By default Kyma cluster-admins will become CF admins | List of users, which will become CF administrators.A user is expected in format sap.ids:\<sap email\> example sap.ids:samir.zeort@sap.com  |

## Dependencies
* **Istio Kyma Module**

  That is normally installed on a SAP managed Kyma
* **Docker Registry**
  
  That is an external docker registry needed for storing/loading application images. If not specified in the CFAPI CR, the CFAPI kyma module deploys a local registry (Dockerregistry CR) which is managed by docker-registry kyma module, see https://github.com/kyma-project/docker-registry. The local docker registry is not suitable for large-scale productive setups.
* **UAA**

  A running UAA server is a must for CFAPI installation. In case of SAP managed Kyma, the UAA is already installed so no additional installation required.
  

## Installation
1. ### Setup a Kyma environment ###

2. ### Istio installed ###

    If Istio Kyma module is not installed, you can do it with:

```
kubectl label namespace cfapi-system istio-injection=enabled --overwrite
kubectl apply -f https://github.com/kyma-project/istio/releases/latest/download/istio-manager.yaml
kubectl apply -f https://github.com/kyma-project/istio/releases/latest/download/istio-default-cr.yaml
```

3. ### [Maybe] deploy docker registry module
In case you want to use an existing docker registry, you do not need to install that.
In all other cases, see https://github.com/kyma-project/docker-registry/blob/main/docs/user/README.md
Installing docker registry operator:
```
kubectl apply -f https://github.com/kyma-project/docker-registry/releases/latest/download/dockerregistry-operator.yaml
```

4. ### Deploy CF API ###

Deploy the resources from a particular release /in the example below 0.2.0/ version to kyma
```
kubectl apply -f https://github.com/kyma-project/cfapi/releases/download/0.2.0/cfapi-manager.yaml
kubectl apply -f https://github.com/kyma-project/cfapi/releases/download/0.2.0/cfapi-default-cr.yaml
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
