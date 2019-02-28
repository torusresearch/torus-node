#!/bin/bash

echo STARTING GAN AND TRUFFLE
ganache-cli -p=14103 -a=20 -s=something > logGan.txt & disown
cd solidity
truffle migrate
cd ..
