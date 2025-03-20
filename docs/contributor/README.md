# CFAPI Kyma Module development

## Environment
* Kyma Environment
  That could be a SAP BTP Managed Kyma environment or an open-source Kyma.
* UAA server
  The UAA server is used in runtime to authenticate users accessing the CF API (via cf cli or direct), not a prerequisite for build and deploy of the module
* Development tools
  - git
  - make
  - golang
  - docker
  - kubectl
* Docker registry
  By default a github docker registry is used, but another registry could be also used. 
## Build
Build is executed via make.Prior to <em>make</em> build some variables shall be set.<br> 
Default values:<br> 
* REGISTRY = ghcr.io
* IMG = kyma-project/cfapi/cfapi-controller
* VERSION = 0.0.0<br>

Important is to change values in such a way that the REGISTRY/IMG/VERSION combination is unique for a particular contributor, so that there are no naming clashes in the target registry.<br>
A contributor could set the variables directly in the makefile (in local git repo) or prior to make command execution:<br>
In the example below the VERSION is personalized for a contributor with name Fabrizio.<br> 
```
VERSION=0.0.fab make docker-build
VERSION=0.0.fab make docker-push
VERSION=0.0.fab make release
```
The result will be a folder rel-0.0.fab/ containing generated kubernetes manfests, which are then ready to deploy in Kyma.<br>

## Deploy
Assume the VERSION variable is from previous example set to 0.0.fab <br>
```
kubectl apply -f 0.0.fab/cfapi-crd.yaml
kubectl apply -f 0.0.fab/cfapi-manager.yaml
kubectl apply -f 0.0.fab/cfapi-default-cr.yaml
```
A contributor might want to create and deploy a custom non-default resource of course.<br>
Check status with:<br>
```
kubectl get -n cfapi-system cfapi
```

