#!/bin/zsh

TESTGROUND_BIN="testground"
CMD="run $TESTCASE $INSTANCES $FILE_SIZE $RUN_COUNT $LATENCY $JITTER $PARALLEL_GEN $LEECH_COUNT $BANDWIDTH $INPUT_DATA $DATA_DIR $TCP_ENABLED $MAX_CONNECTION_RATE $PASSIVE_COUNT $STRATEGY_FUNC $ROUND_SIZE $INITIAL_RATIOS"
# RUNNER="local:exec"
# BUILDER="exec:go"

echo "Starting test..."

run_bitswap(){
    $TESTGROUND_BIN run single \
        --build-cfg skip_runtime_image=true \
        --plan=testbed \
        --testcase=$1 \
        --builder=$BUILDER \
        --runner=$RUNNER --instances=$2 \
        -tp node_type="bitswap" \
        -tp file_size=$3 \
        -tp run_count=$4 \
        -tp latency_ms=$5 \
        -tp jitter_pct=$6 \
        -tp parallel_gen_mb=$7 \
        -tp leech_count=$8 \
        -tp bandwidth_mb=$9 \
        -tp input_data=${10} \
        -tp data_dir=${11} \
        -tp enable_tcp=${12} \
        -tp max_connection_rate=${13} \
        -tp passive_count=${14} \
        -tp strategy_func=${15} \
        -tp round_size=${16} \
        -tp initial_ratios=${17}
}

run() {
    echo "Running test with ($1, $2, $3, $4, $5, $6, $7, $8, $9, ${10}, ${11}, ${12}, ${13}, ${14}, ${15} ${16} ${17}) (TESTCASE, INSTANCES, FILE_SIZE, RUN_COUNT, LATENCY, JITTER, PARALLEL, LEECH, BANDWIDTH, INPUT_DATA, DATA_DIR, TCP_ENABLED, MAX_CONNECTION_RATE, PASSIVE_COUNT, STRATEGY_FUNC, ROUND_SIZE, INITIAL_RATIOS)"
    TESTID=$(run_bitswap $1 $2 $3 $4 $5 $6 $7 $8 $9 ${10} ${11} ${12} ${13} ${14} ${15} ${16} ${17}| tail -n 1 | awk -F 'run is queued with ID:' '{ print $2 }' | tr -d ' ')
    checkstatus $TESTID
    # `run_bitswap $1 $2 $3 $4 $5 $6 $7 $8 $9 ${10} ${11} ${12} ${13} ${14}| tail -n 1 | awk -F 'run with ID: ' '{ print $2 }'`
    # echo $TESTID
    # echo "Finished test $TESTID"
    $TESTGROUND_BIN collect --runner=$RUNNER $TESTID
    tar xzvf $TESTID.tgz
    rm $TESTID.tgz
    local filesizes=$(echo $3 | tr ',' '_')
    local ratios=$(echo ${17} | tr ',' '_')
    local outdir="./results/peer-weights/p${2}-l${8}-f$filesizes-runs${4}-bw${9}-lat${5}-jit${6}-strat_${15}-rs${16}/ratios$ratios"
    mkdir -p $outdir
    mv $TESTID $outdir
    echo "results stored in: $outdir/$TESTID"
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
