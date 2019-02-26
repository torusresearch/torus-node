#!/bin/bash

cd $GOPATH/src/github.com/tendermint/tendermint


#Kill existing containers
docker-compose down

# Clear the build folder
rm -rf ./build

# Build binary
make build-linux

#initialize configs and start docker
docker run -e LOG="stdout" -v `pwd`/build:/tendermint tendermint/localnode testnet --o . --v 5
docker-compose up