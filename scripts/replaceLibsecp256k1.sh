#!/bin/bash
echo COPYING FILES
SCRIPTPATH="$( cd "$(dirname "$0")" ; pwd -P )/.."
echo $SCRIPTPATH/../../ethereum/go-ethereum/crypto/secp256k1/libsecp256k1
echo TO
echo $SCRIPTPATH/vendor/github.com/go-ethereum/crypto/secp256k1/libsecp256k1

cp -R $SCRIPTPATH/../../ethereum/go-ethereum/crypto/secp256k1/libsecp256k1 $SCRIPTPATH/vendor/github.com/ethereum/go-ethereum/crypto/secp256k1
