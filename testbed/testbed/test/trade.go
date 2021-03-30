package test

import (
	"context"
	"errors"
	"fmt"
	"sync"
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
	transferNode := t.node
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

	// Set up network (with traffic shaping)
	if err := utils.SetupNetwork(ctx, runenv, t.nwClient, t.nodetp, t.tpindex, testParams.Latency,
		testParams.Bandwidth, testParams.JitterPct); err != nil {
		return fmt.Errorf("Failed to set up network: %v", err)
	}

	runenv.RecordMessage("Network initialized")

	// Accounts for every file that couldn't be found.
	var fetchFails int64
	publishedRootCids := []cid.Cid{}
	fetchedRootCids := []cid.Cid{}

	runenv.RecordMessage("Network initialized")

	// Wait for all nodes to be ready to start the run
	err = signalAndWaitForAll("start-cid-publish")
	if err != nil {
		return err
	}

	runenv.RecordMessage("Publishing file CIDs...")

	switch t.tpindex {
	case 0: // 0'th peer downloads from  + uploads to everyone

		// publish a single file for all to download
		var publishedCid cid.Cid
		publishedCid, err = t.addPublishFile(ctx, 0, testParams.File, runenv, testvars)
		if err != nil {
			return err
		}
		publishedRootCids = append(publishedRootCids, publishedCid)

		// grab cids to download from all peers
		for i := 1; i < len(testvars.Permutations); i++ {
			var fetchedCid cid.Cid
			fetchedCid, err = t.readFile(ctx, i, runenv, testvars)
			if err != nil {
				return fmt.Errorf("Error fetching cid #%d: %s", i, err.Error())
			}
			runenv.RecordMessage(fmt.Sprintf("Successfuly fetched cid #%d: %s", i, fetchedCid))
			fetchedRootCids = append(fetchedRootCids, fetchedCid)
		}

	default: // other peers publish one file for peer 0, and download one from 0

		// publish a single file for 0 to download
		publishedCid, err := t.addPublishFile(ctx, t.tpindex, testParams.File, runenv, testvars)
		if err != nil {
			return err
		}
		publishedRootCids = append(publishedRootCids, publishedCid)

		// grab cid to download from i
		fetchedCid, err := t.readFile(ctx, 0, runenv, testvars)
		if err != nil {
			return err
		}
		runenv.RecordMessage(fmt.Sprintf("Successfuly fetched cid #0: %s", fetchedCid))
		fetchedRootCids = append(fetchedRootCids, fetchedCid)
	}

	runenv.RecordMessage("File injest complete...")
	// Wait for all nodes to be ready to dial
	err = signalAndWaitForAll("injest-complete")
	if err != nil {
		return err
	}

	runenv.RecordMessage("Starting %s Fetch...", nodeType)

	for runNum := 1; runNum < testvars.RunCount+1; runNum++ {
		// Reset the timeout for each run
		ctx, cancel := context.WithTimeout(ctx, testvars.RunTimeout)
		defer cancel()

		runID := fmt.Sprintf("%d", runNum)

		// Wait for all nodes to be ready to start the run
		err = signalAndWaitForAll("start-run-" + runID)
		if err != nil {
			return err
		}

		runenv.RecordMessage("Starting run %d / %d (%d bytes)", runNum, testvars.RunCount, testParams.File.Size())

		var peersToDial []utils.PeerInfo
		switch t.tpindex {
		case 0: // peer 0 connects to everyone
			for _, peerInfo := range t.peerInfos {
				if peerInfo.TpIndex != 0 {
					peersToDial = append(peersToDial, peerInfo)
				}
			}
		default: // everyone connects to peer 0
			for _, peerInfo := range t.peerInfos {
				if peerInfo.TpIndex == 0 {
					peersToDial = append(peersToDial, peerInfo)
				}
			}
		}

		dialed, err := t.dialFn(ctx, *t.host, t.nodetp, peersToDial, testvars.MaxConnectionRate)
		if err != nil {
			return err
		}
		runenv.RecordMessage("Dialed %d other nodes", len(dialed))

		// Wait for all nodes to be connected
		err = signalAndWaitForAll("connect-complete-" + runID)
		if err != nil {
			return err
		}

		// @dgrisham: we only want to run bitswap tests
		bsnode, ok := t.node.(*utils.BitswapNode)
		if !ok {
			return errors.New("Not a Bitswap node, existing")
		}

		// @dgrisham: set up bitswap ledgers
		for _, peerInfo := range t.peerInfos {

			numBytesSent := getInitialSendTrade(t.tpindex, peerInfo.TpIndex)
			if numBytesSent != 0 {
				runenv.RecordMessage("Setting sent value in ledger to %d bytes for %s %d (peer %s)", numBytesSent, peerInfo.Nodetp, peerInfo.TpIndex, peerInfo.Addr.ID.String())
				bsnode.Bitswap.SetLedgerSentBytes(peerInfo.Addr.ID, int(numBytesSent))
			}

			numBytesRcvd := getInitialSendTrade(peerInfo.TpIndex, t.tpindex)
			if numBytesRcvd != 0 {
				runenv.RecordMessage("Setting received value in ledger to %d bytes for %s %d (peer %s)", numBytesRcvd, peerInfo.Nodetp, peerInfo.TpIndex, peerInfo.Addr.ID.String())
				bsnode.Bitswap.SetLedgerReceivedBytes(peerInfo.Addr.ID, int(numBytesRcvd))
			}
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
						receiptID := fmt.Sprintf("receiptAtTime/peer:%s/sent:%v/recv:%v/value:%v/exchanged:%v", receipt.Peer, receipt.Sent, receipt.Recv, receipt.Value, receipt.Exchanged)
						runenv.R().RecordPoint(receiptID, float64(1))

						// save ledger sends in case there are more runs/files
						setSendTrade(t.tpindex, peerInfo.TpIndex, receipt.Sent)
						setSendTrade(peerInfo.TpIndex, t.tpindex, receipt.Sent)
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

		fetchResults := make([]fetchResult, len(fetchedRootCids))
		var wg sync.WaitGroup
		for fetchIdx, fetchCid := range fetchedRootCids { // download all cids in parallel
			wg.Add(1)

			go func(idx int, cid cid.Cid, wg *sync.WaitGroup) {
				defer wg.Done()

				start := time.Now()
				runenv.RecordMessage("Starting to fetch index #%d, %d / %d (%d bytes)", runNum, idx, testvars.RunCount, testParams.File.Size())

				ctxFetch, cancel := context.WithTimeout(ctx, testvars.RunTimeout/2)
				rcvFile, err := transferNode.Fetch(ctxFetch, cid, t.peerInfos)

				if err != nil { // failure
					runenv.RecordMessage("Error fetching cid %s: %v", cid.String(), err)
					fetchFails++
				} else { // success, save metrics
					timeToFetch := time.Since(start)
					fetchResults[idx] = fetchResult{
						CID:  cid,
						Time: timeToFetch,
					}
					s, _ := rcvFile.Size()
					runenv.RecordMessage("Fetch of %d complete (%d ns)", s, timeToFetch)
				}
				cancel()
			}(fetchIdx, fetchCid, &wg)
		}

		wg.Wait()

		// Wait for all downloads to complete
		err = signalAndWaitForAll("transfer-complete")
		if err != nil {
			return err
		}

		/// --- Report stats
		err = t.emitMetricsTrade(runenv, runNum, nodeType, testParams, fetchResults, tcpFetch, fetchFails, testvars.MaxConnectionRate)
		if err != nil {
			return err
		}
		runenv.RecordMessage("Finishing emitting metrics. Starting to clean...")

		for _, fetchCid := range fetchedRootCids {
			if err := t.cleanupRun(ctx, fetchCid, runenv); err != nil {
				return err
			}
		}
	}
	err = t.close()
	if err != nil {
		return err
	}

	runenv.RecordMessage("Ending testcase")
	return nil
}
