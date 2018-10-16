#!/bin/bash

echo Porting Node List Contract ABIs

solc --abi ./solidity/contracts/NodeList.sol | sed '4q;d' >> NodeList.abi
abigen --abi NodeList.abi --pkg nodelist --out ./solidity/goContracts/NodeList.go
rm ./NodeList.abi
