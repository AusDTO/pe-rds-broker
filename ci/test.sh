#!/bin/bash

set -e
set -x

ORIG_PWD="${PWD}"

# Create our own GOPATH
export GOPATH="${ORIG_PWD}/go"

# Symlink our source dir from inside of our own GOPATH
mkdir -p "${GOPATH}/src/github.com/AusDTO"
ln -s "${ORIG_PWD}/src" "${GOPATH}/src/github.com/AusDTO/pe-rds-broker"
cd "${GOPATH}/src/github.com/AusDTO/pe-rds-broker"

# Cache glide deps
export GLIDE_HOME="${ORIG_PWD}/src/.glide_cache"
mkdir -p "${GLIDE_HOME}"

# Install go deps
glide install

# Build the thing
go build

# Run Go tests - skip for now - TODO enable
#go test $(go list ./... | grep -v "/vendor/")

# Copy artefacts to output directory
cp \
        "${ORIG_PWD}/src/manifest-template.yml" \
        "${ORIG_PWD}/src/Procfile" \
        "${ORIG_PWD}/src/config-govau.yml" \
    "${ORIG_PWD}/build"

cp "${ORIG_PWD}/src/pe-rds-broker" \
   "${ORIG_PWD}/build/rds-broker"

echo "Files in build:"
ls -l "${ORIG_PWD}/build"
