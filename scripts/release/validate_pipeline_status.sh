#!/usr/bin/env bash

echo "Checking status of 'post-*' pipelines for cfapi-operator"
REF_NAME="${1:-"main"}"
STATUS_URL="https://api.github.com/repos/kyma-project/cfapi/commits/${REF_NAME}/status"
STATUS=$(curl -L -H "Accept: application/vnd.github+json" -H "X-GitHub-Api-Version: 2022-11-28" "${STATUS_URL}" | head -n 2 )
if [[ "$STATUS" == *"success"* ]]; then
  echo "All recent jobs succeeded, post-pipelines are green."
else
  echo "Latest post-pipelines are failing or pending! Reason:"
  echo "$STATUS"
  exit 1
fi
