UAA_URL="https://uaa.cf.sap.hana.ondemand.com"
OIDC_PREFIX="sap.ids"

uuidgen() {
  # macos uuidgen generates upper-cased UUIDs. In order to make the script work
  # on both Linux and MacOS, ensure those are lowercased
  bash -c 'uuidgen | tr "[:upper:]" "[:lower:]"'
}

retry() {
  until $@; do
    echo -n .
    sleep 1
  done
  echo
}

login() {
  local btp_user

  echo "Logging in btp"
  if ! btp list accounts/subaccount >/dev/null 2>&1; then
    btp login --sso manual --url https://canary.cli.btp.int.sap
  fi
  btp_user="$(get_btp_user)"
  echo "Logged in BTP as $btp_user"

  btp target
}

get_btp_user() {
  jq -r '.Authentication.Mail' $HOME/.config/.btp/config.json
}

configure_gateway_service() {
  ns="$1"
  service="$2"
  nodePort="$3"

  kubectl patch service -n "$ns" "$service" --patch-file <(
    cat <<EOF
spec:
  ports:
  - name: https
    nodePort: $nodePort
    port: 443
    protocol: TCP
    targetPort: 8443
EOF
  )
}
