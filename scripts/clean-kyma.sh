 #!/bin/bash
 kubectl delete all --all -n cf
 kubectl delete ns cf
 kubectl delete all --all -n korifi
 kubectl delete ns korifi
 kubectl delete all --all -n korifi-gateway
 kubectl delete ns korifi-gateway
 kubectl delete all --all -n kpack
 kubectl delete ns kpack
 kubectl delete deployment localregistry-docker-registry
 kubectl delete all --all -n cfapi-system
 kubectl delete ns cfapi-system
