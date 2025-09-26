#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="${SCRIPT_DIR}/.."
PATH="$PATH":"$ROOT_DIR/bin"
ENVTEST_ASSETS_DIR="${ROOT_DIR}/testbin"
mkdir -p "${ENVTEST_ASSETS_DIR}"
extra_args=()

function configure_integration_tests() {
  "$ROOT_DIR/development/prepare-kind.sh" cfapi
  source "$ROOT_DIR/development/assets/secrets/env/env.sh"

  extra_args+=("--poll-progress-after=3m30s")
}

function configure_non_integration_tests() {
  make -C "$ROOT_DIR" bin/setup-envtest
  source <("$ROOT_DIR/bin/setup-envtest" use -p env --bin-dir "${ENVTEST_ASSETS_DIR}")

  extra_args+=("--poll-progress-after=60s" "--skip-package=integration")
}

function run_ginkgo() {
  if [[ -n "${GINKGO_NODES:-}" ]]; then
    extra_args+=("--procs=${GINKGO_NODES}")
  fi

  if [[ -z "${NON_RECURSIVE_TEST:-}" ]]; then
    extra_args+=("-r")
  fi

  if [[ -n "${UNTIL_IT_FAILS:-}" ]]; then
    extra_args+=("--until-it-fails")
  fi

  if [[ -n "${SEED:-}" ]]; then
    extra_args+=("--seed=${SEED}")
  fi

  if [[ -z "${NO_RACE:-}" ]]; then
    extra_args+=("--race")
  fi

  if [[ -z "${NO_PARALLEL:-}" ]]; then
    extra_args+=("-p")
  fi

  if [[ -z "${KEEP_GOING:-}" ]]; then
    extra_args+=("--keep-going")
  fi

  go run github.com/onsi/ginkgo/v2/ginkgo --output-interceptor-mode=none --randomize-all --randomize-suites "${extra_args[@]}" $@
}

function main() {
  make bin/controller-gen

  if grep -q "tests/integration" <(echo "$@"); then
    configure_integration_tests $@
  else
    configure_non_integration_tests $@
  fi

  run_ginkgo $@
}

main $@
