#!/usr/bin/env bash

pid=$(lsof -i:7545 -t); 

echo "Killing blockchain client process $pid on port 7545"
kill $pid