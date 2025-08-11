This directory contains a set of scripts that facilitate local development of the cfapi kyma module.

The purpose of the scripts is to build a local version of korifi, build a cf api module that references the korifi local version and deploy overything on a kind cluster

### How to use
Make sure you clone the following repos in the same parent directory:
* `git clone git@github.com:cloudfoundry/korifi.git`
* `git clone git@github.com:kyma-project/cfapi.git`
* `git clone git@github.com:kubernetes-sigs/cloud-provider-kind.git`

Run the script:

`cfapi/development/run-kind.sh`
