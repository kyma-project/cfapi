## CFAPI Custom Resource Configuration


### Root Namespace ###
Default value is "cf", recommended is not to change that. The CF API will create all CF resources under that namespace. Namespaces in Kubernetes are flat, but the CF API will create a hierarchy starting from that root namespace. 

### AppImagePullSecret ###
Application docker images are created with kpack and a docker registry is required to store the images. 
That is a name of a dockersecret for the application docker registry. The secret is expected to contain registry credentials with write permissions. 
By default that value is empty and in this case the CFAPI will deploy a local docker registry which is not suitable for productive, but just for a trial scenarios.


### UAA ###
The CF implementation normally needs a UAA server to handle users and authorizations. That is an external dependency and the module will not install UAA, but rather expects that the UAA server is already installed. 
The configuration setting is actually a valid URL of a running UAA server.

By default, if not specified, the CF API will try to find an installed SAP BTP Service Operator and will imply the UAA URL from the operator's configuration.
That is valid normally for SAP BTP managed Kyma environment.
In case of non-managed Kyma, the UAA server is expected to be installed and running. How to install and run UAA is out-of-scope for this product. 

### CFAdmins ###
That is a comma separated list of users which will assume CF admin role. 
By default, if not specified, all cluster-admins of the kyma cluster will become CF admins.



