#!/bin/zsh

RUNNER="local:docker"
BUILDER="docker:go"
# RUNNER="cluster:k8s"
# BUILDER="docker:go"
# RUNNER="local:exec"
# BUILDER="exec:go"

# echo "Cleaning previous results..."

# rm -rf ./results
# mkdir ./results

FILE_SIZE=157286400
# FILE_SIZE=15728640,31457280,47185920,57671680
RUN_COUNT=1
INSTANCES=3
LEECH_COUNT=2
PASSIVE_COUNT=0
LATENCY=10
NODE_TYPE=bitswap
JITTER=10
BANDWIDTH=150
PARALLEL_GEN=100
TESTCASE=transfer
INPUT_DATA=random
DATA_DIR=../testDatasets
TCP_ENABLED=false
MAX_CONNECTION_RATE=100
STRATEGY_FUNC='sigmoid'
ROUND_SIZE=1000000
# INITIAL_RATIOS=0.75,1.0
leech0_ratio=0.5
leech1_ratio=1.0

for ((i=0; i < 9; ++i)); do

    INITIAL_RATIOS="${leech0_ratio},${leech1_ratio}"
    for ((j=0; j < 5; ++j)); do

        source ./exec.sh
        cmd=$(echo $CMD)
        eval $cmd | tee /dev/stderr | ag -o '(?<=results stored in: ).*?$'
        unset cmd out
        docker rm -f testground-redis

        # TODO: see if this is enough to keep used memory down
        docker ps -a -q | xargs docker rm -f
        docker image ls --filter reference=tg-plan-testbed -q | xargs docker rmi -f
        docker volume prune -f
        rm -rf ~/src/ipfs/beyond-bitswap/data
    done

    leech0_ratio=$(printf "%0.2f" $((leech0_ratio+0.1)))
done

tar -cvzf plots.tar.gz plots
