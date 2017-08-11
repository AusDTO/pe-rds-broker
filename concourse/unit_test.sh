#!/bin/bash

set -xe

export GOPATH=$PWD/go
export PATH=$GOPATH/bin:$PATH

go get github.com/onsi/ginkgo/ginkgo github.com/Masterminds/glide
glide install
go build -v -i
ginkgo -r -race
