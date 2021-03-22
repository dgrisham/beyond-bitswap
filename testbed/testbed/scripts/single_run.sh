#!/bin/bash

RUNNER="local:docker"
BUILDER="docker:go"
# RUNNER="cluster:k8s"
# BUILDER="docker:go"
# RUNNER="local:exec"
# BUILDER="exec:go"

echo "Cleaning previous results..."

rm -rf ./results
mkdir ./results

FILE_SIZE=507671680
# FILE_SIZE=15728640,31457280,47185920,57671680
RUN_COUNT=1
INSTANCES=4
LEECH_COUNT=3
PASSIVE_COUNT=0
PARALLEL_GEN=100
TESTCASE=bitswap-transfer
INPUT_DATA=files
DATA_DIR=../../testDataset
TCP_ENABLED=false
MAX_CONNECTION_RATE=100

BANDWIDTH=150 #MB
LATENCY=100 # ms
JITTER=0 # %

source ./exec.sh

eval $CMD

docker rm -f testground-redis
