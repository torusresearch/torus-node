#!/bin/bash


echo KILLING NODES AND GAN ON..
lsof -n -i:14103 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:8000 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:8001 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:8002 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:8003 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:8004 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:8005 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:8006 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:8007 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:8008 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:8009 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:8010 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:8011 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:8012 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:8013 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:8014 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:8015 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:8016 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:8017 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:8018 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:8019 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:26657 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:26660 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:26662 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:26664 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:26666 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:26668 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:26670 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:26672 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:26674 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
lsof -n -i:26676 | grep LISTEN | awk '{ print $2 }' | xargs kill -9
