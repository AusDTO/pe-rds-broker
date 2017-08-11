#!/bin/bash

set -xe

export GOPATH=$PWD/go
export PATH=$GOPATH/bin:$PATH

go get github.com/onsi/ginkgo/ginkgo github.com/Masterminds/glide

cd $GOPATH/src/github.com/AusDTO/pe-rds-broker
glide install
go build -v -i
ginkgo -r -race
