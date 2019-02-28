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
  go run *.go --configPath $SCRIPTPATH/config/config.local.2.json --buildPath $SCRIPTPATH/.build/node2 > log2.txt & disown
  go run *.go --configPath $SCRIPTPATH/config/config.local.3.json --buildPath $SCRIPTPATH/.build/node3 > log3.txt & disown
  go run *.go --configPath $SCRIPTPATH/config/config.local.4.json --buildPath $SCRIPTPATH/.build/node4 > log4.txt & disown
  go run *.go --configPath $SCRIPTPATH/config/config.local.5.json --buildPath $SCRIPTPATH/.build/node5 > log5.txt & disown
  go run *.go --configPath $SCRIPTPATH/config/config.local.1.json --buildPath $SCRIPTPATH/.build/node1 > log1.txt & disown
else
  go run *.go --configPath $SCRIPTPATH/config/config.local.1.json --buildPath $SCRIPTPATH/.build/node1 > /dev/null & disown
  go run *.go --configPath $SCRIPTPATH/config/config.local.2.json --buildPath $SCRIPTPATH/.build/node2 > /dev/null & disown
  go run *.go --configPath $SCRIPTPATH/config/config.local.3.json --buildPath $SCRIPTPATH/.build/node3 > /dev/null & disown
  go run *.go --configPath $SCRIPTPATH/config/config.local.4.json --buildPath $SCRIPTPATH/.build/node4 > /dev/null & disown
  go run *.go --configPath $SCRIPTPATH/config/config.local.5.json --buildPath $SCRIPTPATH/.build/node5 > /dev/null & disown
fi
