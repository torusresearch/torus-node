#!/bin/bash

echo "[INFO] update go.mod and regenerate go.sum"
go mod tidy
go mod vendor

echo "[INFO] fix go.mod not vendoring C libraries"
go get -u github.com/ethereum/go-ethereum@v1.8.20
rm -rf vendor/github.com/ethereum/go-ethereum/crypto
cp -a $GOPATH/pkg/mod/github.com/ethereum/go-ethereum@v1.8.20/crypto vendor/github.com/ethereum/go-ethereum/crypto
chmod -R 755 vendor/github.com/ethereum/go-ethereum/crypto

echo "[INFO] go install"
cd cmd/dkgnode && go install && cd ../..
