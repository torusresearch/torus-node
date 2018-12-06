#!/bin/bash

#have tendermint and docker installed
#get tendermint from github.com/torusresearch/tendermint
cd $GOPATH/src/github.com/tendermint/tendermint

# Build the linux binary in ./build
make build-linux

# Build tendermint/localnode image
cd $GOPATH/src/github.com/tendermint/tendermint/networks/local
make

# Some how needs to be done twice
cd $GOPATH/src/github.com/tendermint/tendermint
make build-linux

