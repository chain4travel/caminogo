#!/usr/bin/env bash

set -euo pipefail

# e.g.,
# ./scripts/tests.upgrade.sh                                                # Use default version
# ./scripts/tests.upgrade.sh 1.1.18                                         # Specify a version
# CAMINOGO_BIN_PATH=./path/to/caminogo ./scripts/tests.upgrade.sh 1.1.18    # Customization of caminogo path
if ! [[ "$0" =~ scripts/tests.upgrade.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

# The CaminoGo local network does not support long-lived
# backwards-compatible networks. When a breaking change is made to the
# local network, this flag must be updated to the last compatible
# version with the latest code.

DEFAULT_VERSION="v1.1.20-rc0"

VERSION="${1:-${DEFAULT_VERSION}}"

if [[ -z "${VERSION}" ]]; then
  echo "Missing version argument!"
  echo "Usage: ${0} [VERSION]" >>/dev/stderr
  exit 255
fi

CAMINOGO_BIN_PATH="$(realpath "${CAMINOGO_BIN_PATH:-./build/caminogo}")"

#################################
# clone caminogo tag/branch
# https://github.com/chain4travel/caminogo.git
GIT_URL=https://github.com/chain4travel/caminogo.git

rm -rf "/tmp/caminogo-${VERSION}"
rm -rf /tmp/caminogo-build

echo "cloning caminogo tag ${VERSION}"
git clone -b "${VERSION}" ${GIT_URL} "/tmp/caminogo-${VERSION}"

find "/tmp/caminogo-${VERSION}"

cd "/tmp/caminogo-${VERSION}"
./scripts/build.sh
cd - 


# Sourcing constants.sh ensures that the necessary CGO flags are set to
# build the portable version of BLST. Without this, ginkgo may fail to
# build the test binary if run on a host (e.g. github worker) that lacks
# the instructions to build non-portable BLST.
source ./scripts/constants.sh

#################################
echo "building upgrade.test"
# to install the ginkgo binary (required for test build and run)
go install -v github.com/onsi/ginkgo/v2/ginkgo@v2.4.0
ACK_GINKGO_RC=true ginkgo build ./tests/upgrade
./tests/upgrade/upgrade.test --help

#################################
# By default, it runs all upgrade test cases!
echo "running upgrade tests against the local cluster with ${CAMINOGO_BIN_PATH}" 
./tests/upgrade/upgrade.test \
  --ginkgo.v \
  --caminogo-path="/tmp/caminogo-${VERSION}/build/caminogo" \
  --caminogo-path-to-upgrade-to="${CAMINOGO_BIN_PATH}"
