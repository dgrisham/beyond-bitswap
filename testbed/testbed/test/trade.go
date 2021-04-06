package test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"

	"github.com/ipfs/go-cid"
	"github.com/protocol/beyond-bitswap/testbed/testbed/utils"
)

// Trade data between peers
func Trade(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	// Test Parameters
	testvars, err := getEnvVars(runenv)
	if err != nil {
		return err
	}
	nodeType := runenv.StringParam("node_type")

	/// --- Set up
	ctx, cancel := context.WithTimeout(context.Background(), testvars.Timeout)
	defer cancel()
	baseT, err := InitializeTest(ctx, runenv, testvars)
	if err != nil {
		return err
	}
	nodeInitializer, ok := supportedNodes[nodeType]
	if !ok {
		return fmt.Errorf("unsupported node type: %s", nodeType)
	}
	t, err := nodeInitializer(ctx, runenv, testvars, baseT)
	// transferNode := t.node
	signalAndWaitForAll := t.signalAndWaitForAll

	// Start still alive process if enabled
	t.stillAlive(runenv, testvars)

	var tcpFetch int64

	// the only thing we'll vary in the permutations is the file, which we want to be unique to each node.
	// all of the peers will be Active peers, so the `tpindex` will be unique for our runs. so we simply
	// index the permutations on the type index to get the unique file for this node + the (constant)
	// bandwidth + latency + jitter options used for all of the permutations
	testParams := testvars.Permutations[t.tpindex]

	runenv.RecordMessage("Initializing network")

	err = signalAndWaitForAll("setup-network")
	if err != nil {
		return err
	}

	// Set up network (with traffic shaping)
	if err := utils.SetupNetwork(ctx, runenv, t.nwClient, t.nodetp, t.tpindex, testParams.Latency,
		testParams.Bandwidth, testParams.JitterPct); err != nil {
		return fmt.Errorf("Failed to set up network: %v", err)
	}

	runenv.RecordMessage("Network initialized")

	// Wait for all nodes to be ready to start the run
	err = signalAndWaitForAll("start-cid-publish")
	if err != nil {
		return err
	}

	// runNum runs for base strategy, then another runNum runs for alt strategy
	totalRuns := testvars.RunCount * 2
	for runNum := 1; runNum <= totalRuns; runNum++ {

		runID := fmt.Sprintf("%d", runNum)

		// publish a single file for all to download
		publishedCid, publishedFilePath, err := t.addPublishFile(ctx, t.tpindex, testParams.File, runenv, testvars)
		if err != nil {
			return err
		}

		runenv.RecordMessage("Published file CID %s", publishedCid.String())

		// Accounts for every file that couldn't be found.
		var fetchFails int64
		fetchedRootCids := make(map[int]cid.Cid)

		// grab cids to download from all peers
		for i := 0; i < len(testvars.Permutations); i++ {
			if i == t.tpindex { // don't grab our own cid
				continue
			}

			fetchedCid, err := t.readFile(ctx, i, runenv, testvars)
			if err != nil {
				return fmt.Errorf("Error fetching cid #%d: %s", i, err.Error())
			}
			runenv.RecordMessage(fmt.Sprintf("(node %d) Successfuly fetched cid #%d: %s", t.tpindex, i, fetchedCid))
			fetchedRootCids[i] = fetchedCid
		}

		runenv.RecordMessage("File injest complete...")
		// Wait for all nodes to be ready to dial
		err = signalAndWaitForAll("injest-complete-" + runID)
		if err != nil {
			return err
		}

		runenv.RecordMessage("Starting %s Fetch...", nodeType)

		// @dgrisham: we only want to run bitswap tests
		bsnode, ok := t.node.(*utils.BitswapNode)
		if !ok {
			return errors.New("Not a Bitswap node, existing")
		}

		// @dgrisham: switch specified user's strategy for half of the runs
		if runNum > totalRuns/2 {
			if t.tpindex == testvars.AltStrategy.User {
				runenv.RecordMessage("User %d is using alternate strategy %s", testvars.AltStrategy.User, testvars.AltStrategy.Strategy)
				bsnode.Bitswap.SetWeightFunc(testvars.AltStrategy.Strategy)
			}
		}
		// Reset the timeout for each run
		ctx, cancel := context.WithTimeout(ctx, testvars.RunTimeout)
		defer cancel()

		// Wait for all nodes to be ready to start the run
		err = signalAndWaitForAll("start-run-" + runID)
		if err != nil {
			return err
		}

		runenv.RecordMessage("Starting run %d / %d (%d bytes)", runNum, testvars.RunCount, testParams.File.Size())

		// dial all peers
		dialed, err := t.dialFn(ctx, *t.host, t.nodetp, t.peerInfos, testvars.MaxConnectionRate)
		if err != nil {
			return err
		}
		runenv.RecordMessage("Dialed %d other nodes", len(dialed))

		// Wait for all nodes to be connected
		err = signalAndWaitForAll("connect-complete-" + runID)
		if err != nil {
			return err
		}

		// @dgrisham: set up bitswap ledgers
		exchangesTrade = copyLedgerData(testvars.InitialSends) // (re)set initial ledger values
		for _, peerInfo := range t.peerInfos {

			numBytesSent := getBytesSentTrade(t.tpindex, peerInfo.TpIndex)
			if numBytesSent != 0 {
				runenv.RecordMessage("Setting sent value in ledger to %d bytes for peer %d (id %s)", numBytesSent, peerInfo.TpIndex, peerInfo.Addr.ID.String())
				bsnode.Bitswap.SetLedgerSentBytes(peerInfo.Addr.ID, int(numBytesSent))
			}

			numBytesRcvd := getBytesSentTrade(peerInfo.TpIndex, t.tpindex)
			if numBytesRcvd != 0 {
				runenv.RecordMessage("Setting received value in ledger to %d bytes for peer %d (id %s)", numBytesRcvd, peerInfo.TpIndex, peerInfo.Addr.ID.String())
				bsnode.Bitswap.SetLedgerReceivedBytes(peerInfo.Addr.ID, int(numBytesRcvd))
			}
		}

		err = signalAndWaitForAll("ledgers-initialized-" + runID)
		if err != nil {
			return err
		}

		// @dgrisham start time series metric gathering functions
		quit := make(chan bool)
		go func() { // record bitswap metrics in the background while fetching blocks

			for {
				select {

				case <-quit: // loop until signal is received
					return

				default:

					for _, peerInfo := range t.peerInfos {
						if peerInfo.Addr.ID == (*(t.host)).ID() {
							continue
						}
						receipt := bsnode.Bitswap.LedgerForPeer(peerInfo.Addr.ID)
						roundReset := bsnode.Bitswap.RoundReset()
						receiptID := fmt.Sprintf("receiptAtTime/run:%d/peer:%s/sent:%v/recv:%v/value:%v/exchanged:%v/weight:%v/workRemaining:%v/roundReset:%t", runNum, receipt.Peer, receipt.Sent, receipt.Recv, receipt.Value, receipt.Exchanged, receipt.Weight, receipt.WorkRemaining, roundReset)
						runenv.R().RecordPoint(receiptID, float64(1))

						// save ledger sends in case there are more runs/files
						setBytesSentTrade(t.tpindex, peerInfo.TpIndex, receipt.Sent)
						setBytesSentTrade(peerInfo.TpIndex, t.tpindex, receipt.Sent)
					}

					time.Sleep(1 * time.Millisecond) // 1 ms between each step
				}
			}
		}()

		// Wait for all nodes
		err = signalAndWaitForAll("background-metric-gathering-started-" + runID)
		if err != nil {
			return err
		}

		/// --- Start test

		fetchCids := []cid.Cid{}
		for _, cid := range fetchedRootCids {
			fetchCids = append(fetchCids, cid)
		}
		ctxFetch, cancel := context.WithTimeout(ctx, testvars.RunTimeout/2)

		runenv.RecordMessage("Fetching cids %v", fetchCids)

		start := time.Now()
		sizes, errs := bsnode.FetchAll(ctxFetch, fetchCids, t.peerInfos)
		timeToFetchAll := time.Since(start)
		for i, err := range errs {
			if err != nil {
				fetchFails++
				runenv.RecordMessage("Error fetching: %s", fetchCids[i].String(), err.Error())
			}
			if fetchFails > 0 {
				cancel()
				return errors.New("Error fetching CID(s)")
			}
		}
		cancel()

		runenv.RecordMessage("Time to fetch all files: %d ns", timeToFetchAll)

		fetchResults := make(map[int]fetchResult, len(fetchedRootCids))
		for i, s := range sizes {
			runenv.RecordMessage("Fetch of %s (%d bytes) complete", fetchCids[i].String(), s)
		}

		// Wait for all downloads to complete
		err = signalAndWaitForAll("transfer-complete-" + runID)
		if err != nil {
			return err
		}

		quit <- true

		/// --- Report stats
		err = t.emitMetricsTrade(runenv, runNum, nodeType, testParams, fetchResults, tcpFetch, fetchFails, testvars.MaxConnectionRate)
		if err != nil {
			return err
		}
		runenv.RecordMessage("Finishing emitting metrics. Starting to clean...")

		if err := t.cleanupRun(ctx, append(fetchCids, publishedCid), runenv); err != nil {
			return err
		}
		if err := os.Remove(publishedFilePath); err != nil {
			return err
		}

		err = signalAndWaitForAll("run-complete-" + runID)
		if err != nil {
			return err
		}
	}
	err = t.close()
	if err != nil {
		return err
	}

	runenv.RecordMessage("Ending testcase")
	return nil
}
