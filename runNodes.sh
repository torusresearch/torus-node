#!/bin/bash

echo RUNNING NODES

go run *.go --configPath /Users/zhen/go/src/github.com/YZhenY/DKGNode/node/config.json & disown
go run *.go --configPath /Users/zhen/go/src/github.com/YZhenY/DKGNode/node/config.1.json  & disown
go run *.go --configPath /Users/zhen/go/src/github.com/YZhenY/DKGNode/node/config.2.json  & disown
go run *.go --configPath /Users/zhen/go/src/github.com/YZhenY/DKGNode/node/config.3.json  & disown
go run *.go --configPath /Users/zhen/go/src/github.com/YZhenY/DKGNode/node/config.4.json  & disown