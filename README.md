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
| ContainerRepositoryPrefix | Optional | `<registryURL>/` | The prefix of the container repository where package and droplet images will be pushed. More details [here](https://github.com/cloudfoundry/korifi/blob/main/INSTALL.md#install-korifi)
| BuilderRepository | Optional | `<registryURL>/cfapi/kpack-builder` | Container image repository to store the kpack `ClusterBuilder` image. More details [here](https://github.com/cloudfoundry/korifi/blob/main/INSTALL.md#install-korifi)
| UAA | Optional | The subaccount UAA |  UAA URL to be used for authentication |
| CFAdmins | Optional | Kyma cluster admins | List of users, which will become CF administrators. A user is expected in format sap.ids:\<sap email\> example sap.ids:samir.zeort@sap.com  |
| UseSelfSignedCertificates | Optional | `false` | Use self signed certificates for CF API and workloads. |
| GatewayType | Optional | `contour` | The underlying gateway api implementation. Accepted values: `contour`, `istio` |

## Dependencies

### Container Image Registry
Used for application source and droplet images. If not configured, the CFAPI module would use the registry provided by the [docker registry module](https://github.com/kyma-project/docker-registry), or issue an error if not enabled.

### UAA
Used to authenticate the user when running `cf` commands, such as `cf push`. If not configured, the CFAPI module would default to the BTP subaccount UAA instance

## Installation
### Setup a Kyma environment

### Docker registry
In the Kyma dashboard:
* Enable the `docker-registry` module
* Make sure external access is enabled

### CFAPI module
In the Kyma dashboard:
* Enable the `cfapi` module.
* Once ready the CF url is set on its status. Keep in mind that it may take up to a couple of minutes for DNS entries to refresh.

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

### Using istio as gateway api implementation

The cfapi module uses contour as a default gateway api implementation since kyma managed istio does not support the gateway api out of the box. To use istio do the following:

In the Kyma dashboard:
* set the module release channel to `exprimental`
* set `spec.experimental.pilot.enableAlphaGatewayAPI` to `true` on the default Istio resource
* set `spec.gatewayType` to `istio` on the default CFAPI resource

### Using a custom docker registry

The cf api module uses the kyma docker registry as container registry. To use a custom container registry (such as dockerhub) do the following:
* Create a container registry secret in the `cfapi-system` namespace, e.g. `dockerhub-secret`:

```
kubectl create secret -n cfapi-system docker-registry dockerhub-secret \
  --docker-server="https://index.docker.io/v1/" \
  --docker-username="<dockerhub-username>" \
  --docker-password="<dockerhub-password>"
```

On the default CFAPI resource:
* Set `spec.containerRegistrySecret` to `dockerhub-secret`
* Set `spec.containerRepositoryPrefix` to `index.docker.io/<organization>/`
* Set `spec.builderRepository` to `index.docker.io/<organization>/kpack-builder`

Referer to [Korifi documentation](https://github.com/cloudfoundry/korifi/blob/main/INSTALL.md#install-korifi) on configuring `containerRepositoryPrefix` and `builderRepository`

## Development

## Contributing
See the [Contributing Rules](CONTRIBUTING.md).

## Code of Conduct
See the [Code of Conduct](CODE_OF_CONDUCT.md) document.

## Licensing

See the [license](./LICENSE) file.
