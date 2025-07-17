# Changes to the minimum golang version must also be replicated in
# scripts/ansible/roles/golang_base/defaults/main.yml
# scripts/build_camino.sh
# scripts/local.Dockerfile
# Dockerfile (here)
# README.md
# go.mod
# ============= Compilation Stage ================
FROM golang:1.19.10-bullseye AS builder
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    git \
    make \
    gcc \
    musl-dev \
    ca-certificates \
    linux-headers-amd64
WORKDIR /build
# Copy and download caminogo dependencies using go mod
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy the code into the container
COPY . .

# Build caminogo and plugins
RUN ./scripts/build.sh
# Build tools
RUN ./scripts/build_tools.sh

# ============= Cleanup Stage ================
FROM debian:11-slim AS execution

# installing wget to get static ip with wget -O - -q icanhazip.com
RUN apt-get update && apt-get install -y wget

# Maintain compatibility with previous images
RUN mkdir -p /caminogo/build
WORKDIR /caminogo/build

# Copy the executables into the container
COPY --from=builder /build/build/ .

CMD [ "./caminogo" ]
