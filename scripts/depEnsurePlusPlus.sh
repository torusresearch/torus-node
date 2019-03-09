#!/bin/bash
echo DEP ENSURING ---------------------------
SCRIPTPATH="$( cd "$(dirname "$0")" ; pwd -P )/.."
dep ensure
$SCRIPTPATH/scripts/replaceLibsecp256k1.sh

echo REMOVING TENDERMINT
rm -rf $SCRIPTPATH/vendor/github.com/tendermint/tendermint
