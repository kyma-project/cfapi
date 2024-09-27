# RELEASE version 0.0.9

# Prerequisites
* UAA set as OIDC provider
* Existing docker registry Or Docker registry kyma module deployed

# In this release 
* Adapt to a change in docker registry module (host instead of hostPrefix)
* API servicebinding.io installed
* CR OpenIDConnect installed in case CRD is found
* CR DockerRegistry installed in case CRD found and docker registry not provided
* Istio experimental Gateway API supported
