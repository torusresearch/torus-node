#!/bin/bash

SCRIPTPATH="$( cd "$(dirname "$0")" ; pwd -P )"

echo CLEARING BUILD FOLDER...
rm -rf $SCRIPTPATH/.build
