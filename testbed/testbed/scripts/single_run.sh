#!/bin/bash

# BRANCH="$1" && shift
# [[ -z "$BRANCH" ]] && BRANCH=master
# git checkout $BRANCH || { echo "error checking out branch '$BRANCH'. exiting" >&2 ; exit 1 ; }
# # echo "Starting test on branch '$BRANCH'..."

# RUNNER="local:docker"
# BUILDER="docker:go"
# RUNNER="cluster:k8s"
# BUILDER="docker:go"
RUNNER="local:exec"
BUILDER="exec:go"

echo "Cleaning previous results..."

rm -rf ./results
mkdir ./results

FILE_SIZE=157286400
# FILE_SIZE=15728640,31457280,47185920,57671680
RUN_COUNT=1
INSTANCES=3
LEECH_COUNT=2
PASSIVE_COUNT=0
LATENCY=10
JITTER=10
BANDWIDTH=150
PARALLEL_GEN=100
TESTCASE=bitswap-transfer
INPUT_DATA=random
TCP_ENABLED=false
MAX_CONNECTION_RATE=100
DATA_DIR='../../test-datasets'
STRATEGY_FUNC='identity'
ROUND_SIZE=10000

source ./exec.sh

echo $CMD
eval $CMD

docker rm -f testground-redis
