This document describes how to build the module that installs a local version of Korifi. The procedure assumes that Korifi repo is cloned next to the cfapi module and required tools (such as kbld) are installed.

### Configuring the Kyma cluster

When a Kyma instance is created in BTP, the user is offered a kubeconfig. This kubeconfig uses the OIDC kubectl plugin to authenticate which unfortunately requires a browser with adequate javascript support. Unfortunately, our headless linux VMs do not have such.

In order to workaround the issue, we have a [script](https://github.tools.sap/unified-runtime/trinity/blob/main/scripts/tools/generate-kyma-kubeconfig.sh) that uses UAA token to authenticate, create an admin service account and generate a kubeconfig for that admin account. We can use that kubeconfig in our stations

However, a prerequisite for that script to run is the cluster to trust the UAA as identity provider. There are two ways to achieve that:
a) configure OIDC parameters in the kyma environment config, or
b) create a `OpenIDConnect` resource

Even though creating the `OpenIDConnect` resource looks nicer, creating it requires a usable kubeconfig, i.e. we cannot automate that via s script that runs on the headless linux VM. That is why we opt for solution `a)` as the less ugly option

Below are the OIDC parameters one should supply when configuring the Kyma cluster:
  - ClientID: `cf`
  - IssuerURL: `https://uaa.cf.sap.hana.ondemand.com/oauth/token`
  - SigningAlgs: `RS256`
  - UsernameClaim: `user_name`
  - UsernamePrefix: `sap.ids:`
  - Administrators: `sap.ids:<your-sap-email-address>` (the `sap.ids:` prefix is very important)

Note that configuring the OIDC config on the kyma cluster breaks the Kyma dashboard (Busola) but that is fine. With a usable kubeconfig we could use k9s instead.

### Install the module

Just run the `korifi-local/create-release.sh` script. It
* installs and configures the kyma docker registry with external access enabled
* builds local korifi and pushes its images to the kyma docker registry
* builds a helm chart archive that references the images just build and packages it into the cfapi module docker image
* deploys the cf api operator and creates a `CFAPI` resource
