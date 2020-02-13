#!/bin/sh

while true
do
    node ../torus-ganache-kube/index.js &> /dev/null &
    pid=$!

    sleep 1

    kill -0 $pid &> /dev/null
    if [[ $? -eq 0 ]]
    then
        break
    fi
done

cd ../torus-website/app
npm run serve &> /dev/null &

sleep 10

xdg-open http://localhost:3000/