#!/usr/bin/env bash

set -x
set -euo pipefail

VERSION=${1:-}
RELEASE_DIR="./release-$VERSION"
GITSHA=$(git rev-parse --short HEAD)

mkdir -p $RELEASE_DIR
cp ./CHANGELOG.md $RELEASE_DIR/
cp ./scripts/assets/korifi-kbld.yml $RELEASE_DIR/

kbld \
      -f "$RELEASE_DIR/korifi-kbld.yml" \
      -f "./helm/korifi/values.yaml" \
      --images-annotation=false >$RELEASE_DIR/values.yaml

API_SOURCE=`yq .api.image $RELEASE_DIR/values.yaml`
CONTR_SOURCE=`yq .controllers.image $RELEASE_DIR/values.yaml
`
API_TARGET="ghcr.io/kyma-project/cfapi/korifi-api"
CONTR_TARGET="ghcr.io/kyma-project/cfapi/korifi-controllers"

echo "tag target API image $API_TARGET"
docker tag $API_SOURCE $API_TARGET:latest
docker tag $API_SOURCE $API_TARGET:$VERSION
docker tag $API_SOURCE $API_TARGET:$GITSHA
docker push --all-tags $API_TARGET

echo "tag target CONTROLLERS image $API_TARGET"
docker tag $CONTR_SOURCE $CONTR_TARGET:latest
docker tag $CONTR_SOURCE $CONTR_TARGET:$VERSION
docker tag $CONTR_SOURCE $CONTR_TARGET:$GITSHA
docker push --all-tags $CONTR_TARGET

export API_SHA_REF=$(docker inspect --format='{{index .RepoDigests 0}}' $API_TARGET)
export CONTR_SHA_REF=$(docker inspect --format='{{index .RepoDigests 0}}' $CONTR_TARGET)

yq -i '.api.image = strenv(API_SHA_REF)' $RELEASE_DIR/values.yaml
yq -i '.controllers.image = strenv(CONTR_SHA_REF)' $RELEASE_DIR/values.yaml


tar -czf $RELEASE_DIR/korifi-helm.tar.gz -C helm korifi