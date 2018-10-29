#!/bin/bash

echo STARTING GAN AND TRUFFLE
ganache-cli -s=something & disown
cd solidity
truffle migrate
cd ..
