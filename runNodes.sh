#!/bin/bash

echo RUNNING NODES
cd cmd/DKGNode
go run *.go --configPath $GOPATH/src/github.com/YZhenY/DKGNode/local/config.1.json  & disown
go run *.go --configPath $GOPATH/src/github.com/YZhenY/DKGNode/local/config.2.json  & disown
go run *.go --configPath $GOPATH/src/github.com/YZhenY/DKGNode/local/config.3.json  & disown
go run *.go --configPath $GOPATH/src/github.com/YZhenY/DKGNode/local/config.4.json  & disown
go run *.go --configPath $GOPATH/src/github.com/YZhenY/DKGNode/local/config.5.json & disown