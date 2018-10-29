#!/bin/bash

echo KILLING NODES ON..
lsof -n -i:8001 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i:8002 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i:8003 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i:8004 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i:8005 | grep LISTEN | awk '{ print $2 }' | xargs kill