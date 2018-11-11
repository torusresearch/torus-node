#!/bin/bash

echo RUNNING NODES
cd cmd
go run *.go --configPath $(dirname $0)/config/config.local.1.json & disown
go run *.go --configPath $(dirname $0)/config/config.local.2.json & disown
go run *.go --configPath $(dirname $0)/config/config.local.3.json & disown
go run *.go --configPath $(dirname $0)/config/config.local.4.json & disown
go run *.go --configPath $(dirname $0)/config/config.local.5.json & disown