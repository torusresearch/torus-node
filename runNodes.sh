#!/bin/bash

echo RUNNING NODES
SCRIPTPATH="$( cd "$(dirname "$0")" ; pwd -P )"
cd cmd/dkgnode
go run *.go --configPath $SCRIPTPATH/config/config.local.1.json & disown
go run *.go --configPath $SCRIPTPATH/config/config.local.2.json & disown
go run *.go --configPath $SCRIPTPATH/config/config.local.3.json & disown
go run *.go --configPath $SCRIPTPATH/config/config.local.4.json & disown
go run *.go --configPath $SCRIPTPATH/config/config.local.5.json & disown