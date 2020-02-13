#!/bin/bash

docker-compose up -d --build
cd ./solidity
truffle migrate
cd ../
docker-compose logs -f
