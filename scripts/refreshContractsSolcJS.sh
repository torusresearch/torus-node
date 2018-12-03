#!/bin/bash

echo Porting Node List Contract ABIs

rm -rf ./temporary
mkdir ./temporary
solcjs --abi ../solidity/contracts/NodeList.sol -o ./temporary
filename=`ls ./temporary | grep NodeList.abi`
abigen -solc=solcjs --abi ./temporary/$filename --pkg nodelist --out ../solidity/goContracts/NodeList.go
rm -rf ./temporary