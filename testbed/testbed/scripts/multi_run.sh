#!/bin/zsh

# RUNNER="local:docker"
# BUILDER="docker:go"
# RUNNER="cluster:k8s"
# BUILDER="docker:go"
RUNNER="local:exec"
BUILDER="exec:go"

# echo "Cleaning previous results..."

# rm -rf ./results
# mkdir ./results

# FILE_SIZE=157286400
FILE_SIZE='100000000,200000000:200000000,200000000:200000000,100000000'
# FILE_SIZE=15728640,31457280,47185920,57671680
RUN_COUNT=100
INSTANCES=3
LEECH_COUNT=0
PASSIVE_COUNT=0
LATENCY=25
NODE_TYPE=bitswap
JITTER=10
BANDWIDTH=512
PARALLEL_GEN=100
TESTCASE=trade
INPUT_DATA=random
DATA_DIR=../testDatasets
TCP_ENABLED=false
MAX_CONNECTION_RATE=100

STRATEGY='constant'
ALT_STRATEGY='1:identity'
RAND_RATIOS=false

ROUND_SIZE=10000000
INITIAL_SCALE=10000000
INITIAL_RATIOS='0:1:0.7,0:2:1.1,1:2:0.8'

source ./exec.sh
cmd=$(echo $CMD)
eval $cmd

# clean up
 # docker ps -a -q | xargs docker rm -f
docker ps -a | grep -v goproxy | tail -n+2 | awk '{print $1}' | xargs docker rm -f
docker image ls --filter reference=tg-plan-testbed -q | xargs docker rmi -f
docker volume prune -f
rm -rf ~/src/ipfs/beyond-bitswap/data

# tar -cvzf plots.tar.gz plots
