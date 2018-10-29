#!/bin/bash

echo RUNNING NODES
cd cmd/DKGNode
go run *.go --configPath /Users/zhen/go/src/github.com/YZhenY/DKGNode/cmd/DKGNode/config.json & disown
go run *.go --configPath /Users/zhen/go/src/github.com/YZhenY/DKGNode/cmd/DKGNode/config.1.json  & disown
go run *.go --configPath /Users/zhen/go/src/github.com/YZhenY/DKGNode/cmd/DKGNode/config.2.json  & disown
go run *.go --configPath /Users/zhen/go/src/github.com/YZhenY/DKGNode/cmd/DKGNode/config.3.json  & disown
go run *.go --configPath /Users/zhen/go/src/github.com/YZhenY/DKGNode/cmd/DKGNode/config.4.json  & disown