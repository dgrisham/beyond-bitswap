#!/bin/bash

TESTGROUND_BIN="testground"
CMD="run $TESTCASE $INSTANCES $FILE_SIZE $RUN_COUNT $PARALLEL_GEN $LEECH_COUNT $INPUT_DATA $DATA_DIR $TCP_ENABLED $MAX_CONNECTION_RATE $PASSIVE_COUNT"
# RUNNER="local:exec"
# BUILDER="exec:go"

run_bitswap() {

    $TESTGROUND_BIN run single \
        --build-cfg skip_runtime_image=true \
        --plan=testbed/testbed \
        --testcase=$1 \
        --builder=$BUILDER \
        --runner=$RUNNER --instances=$2 \
        -tp file_size=$3 \
        -tp run_count=$4 \
        -tp parallel_gen_mb=$5 \
        -tp leech_count=$6 \
        -tp input_data=$7 \
        -tp data_dir=$8 \
        -tp enable_tcp=$9 \
        -tp max_connection_rate=${10} \
        -tp passive_count=${11}
        # --dep github.com/ipfs/go-bitswap=github.com/dgrisham/go-bitswap@peer-weights \
        # --dep github.com/ipfs/go-peertaskqueue=github.com/dgrisham/go-peertaskqueue@peer-weights
        # | tail -n 1 | awk -F 'run with ID: ' '{ print $2 }'

}

run() {
    echo "Running test with ($1, $2, $3, $4, $5, $6, $7, $8, $9, ${10}, ${11}) (TESTCASE, INSTANCES, FILE_SIZE, RUN_COUNT, PARALLEL, LEECH, INPUT_DATA, DATA_DIR, TCP_ENABLED, MAX_CONNECTION_RATE, PASSIVE_COUNT)"
    TESTID=`run_bitswap $1 $2 $3 $4 $5 $6 $7 $8 $9 ${10} ${11} | tail -n 1 | awk -F 'run is queued with ID:' '{ print $2 }'`
    checkstatus $TESTID
    # `run_bitswap $1 $2 $3 $4 $5 $6 $7 $8 $9 ${10} ${11} ${12} ${13} ${14}| tail -n 1 | awk -F 'run with ID: ' '{ print $2 }'`
    # echo $TESTID
    # echo "Finished test $TESTID"
    $TESTGROUND_BIN collect --runner=$RUNNER $TESTID
    tar xzvf $TESTID.tgz
    rm $TESTID.tgz
    local outdir="./results/peer-weights/p${2}-l${6}-f${3}"
    echo "outdir $outdir"
    mkdir -p $outdir
    mv $TESTID $outdir
    echo "Collected results"
}

getstatus() {
    STATUS=`testground status --task $1 | tail -n 2 | awk -F 'Status:' '{ print $2 }'`
    echo ${STATUS//[[:blank:]]/}
}

checkstatus(){
    STATUS="none"
    while [ "$STATUS" != "complete" ]
    do
        STATUS=`getstatus $1`
        echo "Getting status: $STATUS"
        sleep 10s
    done
    echo "Task completed"
}

run_composition() {
    echo "Running composition test for $1"
    TESTID=`testground run composition -f $1 | tail -n 1 | awk -F 'run is queued with ID:' '{ print $2 }'`
    checkstatus $TESTID
    $TESTGROUND_BIN collect --runner=$RUNNER $TESTID
    tar xzvf $TESTID.tgz
    rm $TESTID.tgz
    mv $TESTID ./results/
    echo "Collected results"
}

# checkstatus bub74h523089p79be5ng
