#!/bin/bash


echo KILLING NODES AND GAN ON..
lsof -n -i:8545 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i:8001 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i:8002 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i:8003 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i:8004 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i:8005 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i:8010 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i:8011 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i:8012 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i:8013 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i:8014 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i:26657 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i:26660 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i:26662 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i:26664 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i:26666 | grep LISTEN | awk '{ print $2 }' | xargs kill

