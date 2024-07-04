#!/usr/bin/env bash

set -euo pipefail

print_usage() {
  printf "Usage: build [OPTIONS]

  Build caminogo

  Options:

    -r  Build with race detector
"
}

race=''
while getopts 'r' flag; do
  case "${flag}" in
    r) race='-r' ;;
    *) print_usage
      exit 1 ;;
  esac
done

# Caminogo root folder
CAMINO_PATH=$( cd "$( dirname "${BASH_SOURCE[0]}" )"; cd .. && pwd )
# Load the constants
source "$CAMINO_PATH"/scripts/constants.sh

# Download dependencies
echo "Downloading dependencies..."
(cd "$CAMINOGO_PATH" && go mod download)

build_args="$race"

# Build caminogo
"$CAMINO_PATH"/scripts/build_camino.sh $build_args

# Exit build successfully if the CaminoGo binary is created successfully
if [[ -f "$caminogo_path" ]]; then
        echo "Build Successful"
        exit 0
else
        echo "Build failure" >&2
        exit 1
fi
