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
    btp login --sso --url https://canary.cli.btp.int.sap
  fi
  btp_user="$(get_btp_user)"
  echo "Logged in BTP as $btp_user"

  btp target
}

get_btp_user() {
  jq -r '.Authentication.Mail' $HOME/.config/.btp/config.json
}
