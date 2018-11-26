#!/bin/bash

cd $GOPATH/src/github.com/tendermint/tendermint

# Clear the build folder
rm -rf ./build

# Build binary
make build-linux

#initialize configs and start docker
make localnet-start