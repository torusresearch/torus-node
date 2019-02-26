#!/bin/bash

echo RUNNING NODES
SCRIPTPATH="$( cd "$(dirname "$0")" ; pwd -P )"
cd cmd/dkgnode
go run *.go --configPath $SCRIPTPATH/config/config.local.1.json > log1.txt & disown
go run *.go --configPath $SCRIPTPATH/config/config.local.2.json > log2.txt & disown
go run *.go --configPath $SCRIPTPATH/config/config.local.3.json > log3.txt & disown
go run *.go --configPath $SCRIPTPATH/config/config.local.4.json > log4.txt & disown
go run *.go --configPath $SCRIPTPATH/config/config.local.5.json > log5.txt & disown