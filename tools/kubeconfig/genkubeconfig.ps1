$PWD = (Get-Item .).FullName
$SCRIPTDIR = $PSScriptRoot

kubectl apply -f $SCRIPTDIR/serviceaccount.yaml
kubectl wait --for=jsonpath='{.data.token}' secret/admin-serviceaccount
$SA_TOKEN=$(kubectl get secret admin-serviceaccount -o=go-template='{{.data.token | base64decode}}')
cp ~/.kube/config kubeconfig-sa.yaml 
docker run -v ${PWD}:/workdir -ti mikefarah/yq -i ".users |= [{\`"name\`":\`"admin-serviceaccount\`", \`"user\`": {\`"token\`":\`"${SA_TOKEN}\`"}}]" kubeconfig-sa.yaml
docker run -v ${PWD}:/workdir -ti mikefarah/yq -i ".contexts[0].context.user |= \`"admin-serviceaccount\`"" kubeconfig-sa.yaml
