#!/usr/bin/env bash

set -euo pipefail

# Caminogo root folder
CAMINOGO_PATH=$( cd "$( dirname "${BASH_SOURCE[0]}" )"; cd .. && pwd )
# Load the constants
source "$CAMINOGO_PATH"/scripts/constants.sh

LDFLAGS="-X github.com/ava-labs/avalanchego/version.GitCommit=$git_commit"
LDFLAGS="$LDFLAGS -X github.com/ava-labs/avalanchego/version.GitVersion=$git_tag"
LDFLAGS="$LDFLAGS -X github.com/ava-labs/coreth/plugin/evm.GitCommit=$caminoethvm_commit"
LDFLAGS="$LDFLAGS -X github.com/ava-labs/coreth/plugin/evm.Version=$caminoethvm_tag"
LDFLAGS="$LDFLAGS $static_ld_flags"

echo "Building tmpnetctl..."
go build -ldflags "$LDFLAGS"\
   -o "$CAMINOGO_PATH/build/tmpnetctl"\
   "$CAMINOGO_PATH/tests/fixture/tmpnet/cmd/"*.go
