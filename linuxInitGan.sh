#!/bin/bash

echo STARTING GAN AND TRUFFLE
nodejs ./solidity/node_modules/ganache-cli/cli.js -p=8545 -a=20 -s=something & disown
cd solidity
truffle migrate
cd ..