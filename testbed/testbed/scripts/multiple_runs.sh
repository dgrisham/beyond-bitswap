
#!/bin/bash

# RUNNER="cluster:k8s"
# BUILDER="docker:go"

RUNNER="local:docker"
BUILDER="docker:go"

# RUNNER="local:exec"
# BUILDER="exec:go"

# echo "Cleaning previous results..."

# rm -rf ./results
# mkdir ./results

FILE_SIZE=1572864000
# FILE_SIZE=15728640,31457280,47185920,57671680
RUN_COUNT=2
INSTANCES=2
LEECH_COUNT=1
PASSIVE_COUNT=0
PARALLEL_GEN=100
TESTCASE=bitswap-transfer
INPUT_DATA=files
TCP_ENABLED=false
MAX_CONNECTION_RATE=100
DATA_DIR='../../test-datasets/random-1GB'
STRATEGY_FUNC='identity'
ROUND_SIZE=25000

BANDWIDTH=1500 #MB
LATENCY=50 # ms
JITTER=0 # %

source ./exec.sh

echo $CMD
eval $CMD

docker rm -f testground-redis

INSTANCES=3
LEECH_COUNT=2

source ./exec.sh

echo $CMD
eval $CMD

docker rm -f testground-redis
