#!/usr/bin/env bash

# Exit script as soon as a command fails.
set -o errexit

# Get the choice of client: ganache-cli is default
bc_client="ganache-cli"

echo "Chosen client $bc_client"

bc_client_port=7545

start_ganache() {
  node_modules/.bin/ganache-cli --noVMErrorsOnRPCResponse --port=7545 >/dev/null 2>&1 &
}

echo "Starting our own $bc_client client instance at port $bc_client_port"
  start_ganache