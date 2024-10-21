# RELEASE version 0.1.1

# Prerequisites
* Kyma environment has OIDC feature, OpenIDConnect CRD, enabled mid-September 2024
* Existing docker registry Or Docker registry kyma module deployed

# In this release 
* Adapt to a change in docker registry module (host instead of hostPrefix)
* API servicebinding.io installed
* CR OpenIDConnect used
* CR DockerRegistry installed in case CRD found and docker registry not provided
* Istio experimental Gateway API supported
