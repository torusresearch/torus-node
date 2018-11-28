#!/bin/bash

echo RUNNING NODES
SCRIPTPATH="$( cd "$(dirname "$0")" ; pwd -P )"
cd cmd/dkgnode
go run *.go --configPath $SCRIPTPATH/config/config.local.1.json --buildPath $SCRIPTPATH/.build/node0 & disown
go run *.go --configPath $SCRIPTPATH/config/config.local.2.json --buildPath $SCRIPTPATH/.build/node1 > log2.txt & disown
go run *.go --configPath $SCRIPTPATH/config/config.local.3.json --buildPath $SCRIPTPATH/.build/node2 > log3.txt & disown
go run *.go --configPath $SCRIPTPATH/config/config.local.4.json --buildPath $SCRIPTPATH/.build/node3 > log4.txt & disown
go run *.go --configPath $SCRIPTPATH/config/config.local.5.json --buildPath $SCRIPTPATH/.build/node4 > log5.txt & disown