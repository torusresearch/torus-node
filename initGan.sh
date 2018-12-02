#!/bin/bash

echo STARTING GAN AND TRUFFLE
ganache-cli -p=8545 -a=20 -s=something & disown
cd solidity
truffle migrate
cd ..
