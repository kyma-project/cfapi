[![REUSE status](https://api.reuse.software/badge/github.com/kyma-project/cfapi)](https://api.reuse.software/info/github.com/kyma-project/cfapi)

# CFAPI Kyma Module

## Overview
CF API Kyma Module is providing a CF API to run on top of Kyma, using the open source [Korifi](https://github.com/cloudfoundry/korifi) project
Once installed, one could use the cf cli to connect and deploy workloads.

## Custom Resource (CR) Specification
| Property | Optional | Default | Description |
|-----|-----|-----|-----|
| RootNamespace | Optional | `cf` | Root namespace for CF resources |
| ContainerRegistrySecret | Optional | `dockerregistry-config-external` | Container registry secret used to push application images. It has to be of type `docker-registry`  |
| ContainerRepositoryPrefix | Optional | The prefix of the container repository where package and droplet images will be pushed. More details [here](https://github.com/cloudfoundry/korifi/blob/main/INSTALL.md#install-korifi)
| BuilderRepository | Optional | Container image repository to store the kpack `ClusterBuilder` image. More details [here](https://github.com/cloudfoundry/korifi/blob/main/INSTALL.md#install-korifi)
| UAA | Optional | In case of a SAP managed Kyma, the UAA will be derived from the BTP service operator config |  UAA URL to be used for authentication |
| CFAdmins | Optional | By default Kyma cluster-admins will become CF admins | List of users, which will become CF administrators. A user is expected in format sap.ids:\<sap email\> example sap.ids:samir.zeort@sap.com  |
| UseSelfSignedCertificates | Optional | Use self signed certificates for CF API and workloads. Defaults to `false`. |

## Dependencies

### Istio Kyma Module
The istio kyma module is usually installed on a kyma system. However, we require that its exprimental `alphaGatewayAPI` feature is enabled. In order to do that, first set the istio moduel channel to `experimental`, and then set its `spec.experimental.pilot.enableAlphaGatewayAPI` to `true`

### Container Image Registry
Used for application source and droplet images. If not configured, the CFAPI module would use the registry provided by the [docker registry module](https://github.com/kyma-project/docker-registry), or issue an error if not enabled.

### UAA
Used to authenticate the user when running `cf` commands, such as `cf push`. If not configured, the CFAPI module would default to the BTP subaccount UAA instance

## Installation
### Setup a Kyma environment

### Istio Module

#### Managed Istio module
In the Kyma dashboard
* set the module release channel to `exprimental`
* set `spec.experimental.pilot.enableAlphaGatewayAPI` to `true` on the default Istio resource
* make sure the module eventually becomes ready

#### Manual installation
```bash
kubectl label namespace cfapi-system istio-injection=enabled --overwrite
kubectl apply -f https://github.com/kyma-project/istio/releases/latest/download/istio-manager-experimental.yaml
kubectl apply -f https://github.com/kyma-project/istio/releases/latest/download/istio-default-cr.yaml
```

Then enable the GatewayAPI aplha support:

```bash
kubectl -n kyma-system patch  istios.operator.kyma-project.io default --type=merge -p='{"spec":{"experimental": {"pilot": {"enableAlphaGatewayAPI": true}}}}'
```

### Docker registry

#### Managed Docker Registry module
In the Kyma dashboard, enable the `docker-registry` module and enable its external access. Make sure it eventually becomes ready.

#### Manual installation
```bash
kubectl apply -f https://github.com/kyma-project/docker-registry/releases/latest/download/dockerregistry-operator.yaml
kubectl apply -f https://github.com/kyma-project/docker-registry/releases/latest/download/default-dockerregistry-cr.yaml
```

Then enable the registry external access:

```bash
kubectl -n kyma-system patch dockerregistries.operator.kyma-project.io default --type=merge -p='{"spec":{"externalAccess": {"enabled": true}}}'```


### CFAPI module
#### Managed CFAPI module
In the Kyma dashboard, enable the `cfapi` module and make sure it eventually becomes ready. Once ready the CF url is set on its status.


#### Manual installation

```bash
kubectl create namespace cfapi-system
kubectl apply -f https://github.com/kyma-project/cfapi/releases/latest/download/cfapi-operator.yaml
kubectl apply -f https://github.com/kyma-project/cfapi/releases/latest/download/cfapi-default-cr.yaml
```

Make sure that the CFAPI resource eventually becomes ready. Once that is true, the CF url is set on its status:

```bash
kubectl -n cfapi-system get cfapis.operator.kyma-project.io default-cf-api

NAME             STATE   URL
default-cf-api   Ready   https://cfapi.kind-127-0-0-1.nip.io
```

### CF login
```bash
cf login --sso -a <cfapi-url>
```

## Usage

Use CF cli to deploy applications as on a normal CF. The buildpacks used are native community buildpacks.

### Consuming BTP Managed Services

The CFAPI module installs a service broker that allows users consume BTP managed services in their applications. To view available services, run

```
cf service-access
```

By default the services are not enabled. To enable a service (for example `xsuaa`), run

```
cf enable-service-access xsuaa -p application
```

The service above is now available in the marketplace:

```
cf marketplace
Getting all service offerings from marketplace in org org / space space

offering   plans         description                                                          broker
xsuaa      application   Manage application authorizations and trust to identity providers.   BTP
```

See the [documentation](https://docs.cloudfoundry.org/devguide/services/managing-services.html) on how to create a service instance and bind applications to instances.

## Development

## Contributing
See the [Contributing Rules](CONTRIBUTING.md).

## Code of Conduct
See the [Code of Conduct](CODE_OF_CONDUCT.md) document.

## Licensing

See the [license](./LICENSE) file.
