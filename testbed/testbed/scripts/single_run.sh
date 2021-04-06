#!/bin/bash

# RUNNER="local:docker"
# BUILDER="docker:go"
# RUNNER="cluster:k8s"
# BUILDER="docker:go"
RUNNER="local:exec"
BUILDER="exec:go"

echo "Cleaning previous results..."

# rm -rf ./results
# mkdir ./results

# FILE_SIZE=0
FILE_SIZE=5000000,5000000,5000000
RUN_COUNT=1
INSTANCES=3
LEECH_COUNT=0
ACTIVE_COUNT=2
PASSIVE_COUNT=0
LATENCY=10
NODE_TYPE=bitswap
JITTER=10
BANDWIDTH=150
PARALLEL_GEN=100
TESTCASE=trade
INPUT_DATA=random
DATA_DIR=../testDatasets
TCP_ENABLED=false
MAX_CONNECTION_RATE=100
STRATEGY_FUNC='identity'
ROUND_SIZE=10000
INITIAL_RATIOS=1.0,1.0

source ./exec.sh

echo $CMD
eval $CMD

docker rm -f testground-redis
