#!/bin/bash

echo STARTING GAN AND TRUFFLE
ganache-cli -p=8545 -s=something & disown
cd solidity
truffle migrate
cd ..
