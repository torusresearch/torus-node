#!/bin/bash

echo RUNNING NODES
SCRIPTPATH="$( cd "$(dirname "$0")" ; pwd -P )"
cd cmd/dkgnode
go run *.go --configPath $SCRIPTPATH/config/config.local.6.json --buildPath $SCRIPTPATH/.build/node6 > log6.txt & disown
go run *.go --configPath $SCRIPTPATH/config/config.local.7.json --buildPath $SCRIPTPATH/.build/node7 > log7.txt & disown
go run *.go --configPath $SCRIPTPATH/config/config.local.8.json --buildPath $SCRIPTPATH/.build/node8 > log8.txt & disown
go run *.go --configPath $SCRIPTPATH/config/config.local.9.json --buildPath $SCRIPTPATH/.build/node9 > log9.txt & disown
go run *.go --configPath $SCRIPTPATH/config/config.local.10.json --buildPath $SCRIPTPATH/.build/node10 > log10.txt & disown