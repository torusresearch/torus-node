#!/bin/bash

silent=false

while getopts 'hs' flag; do
  case "${flag}" in
    h) echo "-s discards all logs" ;;
    s) silent=true ;;
  esac
done

echo RUNNING NODES
SCRIPTPATH="$( cd "$(dirname "$0")" ; pwd -P )/.."
cd cmd/dkgnode
if [ $silent = false ]; then
  go run *.go --configPath $SCRIPTPATH/config/config.local.scripts.1.json --basePath $SCRIPTPATH/.build/node1 > log1.txt & disown
  go run *.go --configPath $SCRIPTPATH/config/config.local.scripts.2.json --basePath $SCRIPTPATH/.build/node2 > log2.txt & disown
  go run *.go --configPath $SCRIPTPATH/config/config.local.scripts.3.json --basePath $SCRIPTPATH/.build/node3 > log3.txt & disown
  go run *.go --configPath $SCRIPTPATH/config/config.local.scripts.4.json --basePath $SCRIPTPATH/.build/node4 > log4.txt & disown
  go run *.go --configPath $SCRIPTPATH/config/config.local.scripts.5.json --basePath $SCRIPTPATH/.build/node5 > log5.txt & disown
else
  go run *.go --configPath $SCRIPTPATH/config/config.local.scripts.1.json --basePath $SCRIPTPATH/.build/node1 > /dev/null & disown
  go run *.go --configPath $SCRIPTPATH/config/config.local.scripts.2.json --basePath $SCRIPTPATH/.build/node2 > /dev/null & disown
  go run *.go --configPath $SCRIPTPATH/config/config.local.scripts.3.json --basePath $SCRIPTPATH/.build/node3 > /dev/null & disown
  go run *.go --configPath $SCRIPTPATH/config/config.local.scripts.4.json --basePath $SCRIPTPATH/.build/node4 > /dev/null & disown
  go run *.go --configPath $SCRIPTPATH/config/config.local.scripts.5.json --basePath $SCRIPTPATH/.build/node5 > /dev/null & disown
fi
