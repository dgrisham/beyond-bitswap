#!/bin/bash

RUNNER="local:docker"
BUILDER="docker:go"


echo "Cleaning previous results..."

rm -rf ../results
mkdir ../results

FILE_SIZE=15728640,31457280,47185920,57671680
RUN_COUNT=5
INSTANCES=15
LEECH_COUNT=10
PASSIVE_COUNT=0
LATENCY=5
JITTER=10
BANDWIDTH=100
PARALLEL_GEN=100
TESTCASE=ipfs-transfer
INPUT_DATA=random
DATA_DIR=../extra/inputData
TCP_ENABLED=false
MAX_CONNECTION_RATE=10

source ./exec.sh

eval $CMD

docker rm -f testground-redis


FILE_SIZE=15728640,31457280,47185920,57671680
RUN_COUNT=5
INSTANCES=15
LEECH_COUNT=10
PASSIVE_COUNT=0
LATENCY=5
JITTER=10
BANDWIDTH=100
PARALLEL_GEN=100
TESTCASE=ipfs-transfer
INPUT_DATA=random
DATA_DIR=../extra/inputData
TCP_ENABLED=false
MAX_CONNECTION_RATE=50

source ./exec.sh

eval $CMD

docker rm -f testground-redis

FILE_SIZE=15728640,31457280,47185920,57671680
RUN_COUNT=5
INSTANCES=15
LEECH_COUNT=10
PASSIVE_COUNT=0
LATENCY=5
JITTER=10
BANDWIDTH=100
PARALLEL_GEN=100
TESTCASE=ipfs-transfer
INPUT_DATA=random
DATA_DIR=../extra/inputData
TCP_ENABLED=false
MAX_CONNECTION_RATE=100

source ./exec.sh

eval $CMD