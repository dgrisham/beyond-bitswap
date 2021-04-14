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
	files "github.com/ipfs/go-ipfs-files"
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

	runenv.RecordMessage("Network initialized")

	type fileInfo struct {
		node      *files.Node
		path      string
		peerIndex int
		cid       cid.Cid
	}
	generatedFiles := make(map[int]*fileInfo) // fIndex -> fileInfo
	for _, altStrategy := range testvars.AltStrategy.Strategies {
		// runNum runs for base strategy, then another runNum runs for alt strategy
		for runNum := 1; runNum <= testvars.RunCount; runNum++ {

			runID := fmt.Sprintf("%s-%d", altStrategy, runNum)

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

			err = signalAndWaitForAll("setup-network-" + runID)
			if err != nil {
				return err
			}

			// Set up network (with traffic shaping)
			if err := utils.SetupNetwork(ctx, runenv, t.nwClient, t.nodetp, t.tpindex, testParams.Latency,
				testParams.Bandwidth, testParams.JitterPct); err != nil {
				return fmt.Errorf("Failed to set up network: %v", err)
			}

			// Wait for all nodes to be ready to start the run
			err = signalAndWaitForAll("start-run-" + runID)
			if err != nil {
				return err
			}

			runenv.RecordMessage("Starting run %d / %d", runNum, testvars.RunCount)

			// non-alt-strategy users publish a small file for the alt strategy user, and a large one for the rest of the users

			// 0 -> 1 : 1
			// 0 -> 2 : 10
			// 1 -> 0 : 10
			// 1 -> 2 : 10
			// 2 -> 0 : 10
			// 2 -> 1 : 1

			err = signalAndWaitForAll("start-file-generation-" + runID)
			if err != nil {
				return err
			}

			runenv.RecordMessage("First run, generating files...")

			peerIndex := 0
			for _, file := range testParams.Files {
				if peerIndex == t.tpindex {
					peerIndex += 1
				}
				fIndex := t.tpindex*10 + peerIndex

				// NOTE: old code to re-use existing file. doesn't work for some reason
				// if fileInfo, ok := generatedFiles[fIndex]; ok {

				// 	fileNode, err := utils.GetUnixFsNode(fileInfo.path)
				// 	if err != nil {
				// 		return err
				// 	}
				// 	gFile := generatedFiles[fIndex]
				// 	gFile.node = &fileNode
				// 	// generatedFiles[fIndex].node = &fileNode

				// 	runenv.RecordMessage("Found existing file for peer %d, fIndex %d, at path %s", peerIndex, fIndex, fileInfo.path)
				// 	continue
				// }

				fileNode, filePath, err := generateFile(runenv, file)
				if err != nil {
					return err
				}

				generatedFiles[fIndex] = &fileInfo{
					node:      &fileNode,
					path:      filePath,
					peerIndex: peerIndex,
				}

				runenv.RecordMessage("Generated file for peer %d, fIndex %d at path", peerIndex, fIndex, filePath)

				peerIndex += 1
			}

			err = signalAndWaitForAll("start-cid-publish-" + runID)
			if err != nil {
				return err
			}

			// publishedRootCids := make(map[int]cid.Cid, len(testParams.Files))
			{
				for fIndex, file := range generatedFiles {
					defer (*file.node).Close()
					cid, err := t.addFile(ctx, fIndex, *file.node, runenv, testvars)
					if err != nil {
						return err
					}

					file.cid = cid

					runenv.RecordMessage("Published file for peer %d, CID %s, fIndex %d", file.peerIndex, cid.String(), fIndex)
				}
			}

			err = signalAndWaitForAll("start-cid-fetch-" + runID)
			if err != nil {
				return err
			}

			// Accounts for every file that couldn't be found.
			var fetchFails int64
			fetchedRootCids := make(map[int]cid.Cid)

			// grab cids to download from all peers
			{
				peerIndex := 0
				for i := 0; i < len(testvars.Permutations); i++ {
					if i == t.tpindex { // don't grab our own cid
						peerIndex += 1
						continue
					}

					fIndex := peerIndex*10 + t.tpindex
					fetchedCid, err := t.readFile(ctx, fIndex, runenv, testvars)
					if err != nil {
						return fmt.Errorf("Error fetching cid with fIndex %d: %s", fIndex, err.Error())
					}
					runenv.RecordMessage("(node %d) Successfuly fetched cid from peer %d, CID %s, fIndex %d", t.tpindex, peerIndex, fetchedCid, fIndex)
					fetchedRootCids[fIndex] = fetchedCid

					peerIndex += 1
				}
			}

			runenv.RecordMessage("File injest complete...")
			// Wait for all nodes to be ready to dial
			err = signalAndWaitForAll("injest-complete-" + runID)
			if err != nil {
				return err
			}

			// @dgrisham: we only want to run bitswap tests
			bsnode, ok := t.node.(*utils.BitswapNode)
			if !ok {
				return errors.New("Not a Bitswap node, existing")
			}

			if t.tpindex == testvars.AltStrategy.User {
				runenv.RecordMessage("User %d is using alternate strategy %s", testvars.AltStrategy.User, altStrategy)
				bsnode.Bitswap.SetWeightFunc(altStrategy)
			}

			// Reset the timeout for each run
			ctx, cancel := context.WithTimeout(ctx, testvars.RunTimeout)
			defer cancel()

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
			// quit := make(chan bool)
			// go func() { // record bitswap metrics in the background while fetching blocks

			// 	for {
			// 		select {

			// 		case <-quit: // loop until signal is received
			// 			return

			// 		default:

			// 			for _, peerInfo := range t.peerInfos {
			// 				if peerInfo.Addr.ID == (*(t.host)).ID() {
			// 					continue
			// 				}
			// 				receipt := bsnode.Bitswap.LedgerForPeer(peerInfo.Addr.ID)
			// 				roundReset := bsnode.Bitswap.RoundReset()
			// 				receiptID := fmt.Sprintf("receiptAtTime/run:%d/peer:%s/sent:%v/recv:%v/value:%v/exchanged:%v/weight:%v/workRemaining:%v/roundReset:%t", runNum, receipt.Peer, receipt.Sent, receipt.Recv, receipt.Value, receipt.Exchanged, receipt.Weight, receipt.WorkRemaining, roundReset)
			// 				runenv.R().RecordPoint(receiptID, float64(1))

			// 				// save ledger sends in case there are more runs/files
			// 				setBytesSentTrade(t.tpindex, peerInfo.TpIndex, receipt.Sent)
			// 				setBytesSentTrade(peerInfo.TpIndex, t.tpindex, receipt.Sent)
			// 			}

			// 			time.Sleep(1 * time.Millisecond) // 1 ms between each step
			// 		}
			// 	}
			// }()

			// Wait for all nodes
			// err = signalAndWaitForAll("background-metric-gathering-started-" + runID)
			// if err != nil {
			// 	return err
			// }

			/// --- Start test

			fetchCids := []cid.Cid{}
			for _, cid := range fetchedRootCids {
				fetchCids = append(fetchCids, cid)
			}
			ctxFetch, fetchCancel := context.WithTimeout(ctx, testvars.RunTimeout/2)
			defer fetchCancel()

			runenv.RecordMessage("Fetching cids %v", fetchCids)

			err = signalAndWaitForAll("ready-to-fetch-" + runID)
			if err != nil {
				return err
			}

			fetchResults := make(map[int]fetchResult, len(fetchedRootCids))
			// if t.tpindex == testvars.AltStrategy.User {

			start := time.Now()
			sizes, errs := bsnode.FetchAll(ctxFetch, fetchCids, t.peerInfos)
			timeToFetchAll := time.Since(start)
			for i, err := range errs {
				if err != nil {
					fetchFails++
					runenv.RecordMessage("Error fetching: %s", fetchCids[i].String(), err.Error())
				}
				if fetchFails > 0 {
					return errors.New("Error fetching CID(s)")
				}
			}
			cancel()

			runenv.RecordMessage("Time to fetch all files: %d ns", timeToFetchAll)

			for i, s := range sizes {
				runenv.RecordMessage("Fetch of %s (%d bytes) complete", fetchCids[i].String(), s)
			}
			// } else {
			// 	go func() {
			// 		bsnode.FetchAll(ctxFetch, fetchCids, t.peerInfos)
			// 		runenv.RecordMessage("Fetch completed or cancelled")
			// 	}()
			// }

			// Wait for all downloads to complete (only care that the user of interest finished)
			err = signalAndWaitForAll("transfer-complete-" + runID)
			if err != nil {
				return err
			}
			// fetchCancel()

			// quit <- true

			/// --- Report stats
			if t.tpindex == testvars.AltStrategy.User {
				err = t.emitMetricsTrade(runenv, runNum, nodeType, testParams, fetchResults, tcpFetch, fetchFails, testvars.MaxConnectionRate)
				if err != nil {
					return err
				}
				runenv.RecordMessage("Finishing emitting metrics. Starting to clean...")
			}

			// publishCids := make([]cid.Cid, len(publishedRootCids))
			// for _, cid := range publishedRootCids {
			// 	publishCids = append(publishCids, cid)
			// }
			publishedCids := make([]cid.Cid, len(generatedFiles))
			for _, f := range generatedFiles {
				publishedCids = append(publishedCids, f.cid)
			}
			allCids := append(fetchCids, publishedCids...)

			runenv.RecordMessage("cids to clean up: %v", allCids)

			if err := t.cleanupRun(ctx, allCids, runenv); err != nil { // disconnect from all peers
				return err
			}

			err = bsnode.DAGService().RemoveMany(ctx, allCids)
			if err != nil {
				return err
			}

			for _, fileInfo := range generatedFiles {
				runenv.RecordMessage("Removing file %s", fileInfo.path)
				if err := os.Remove(fileInfo.path); err != nil {
					return err
				}
			}

			err = bsnode.ClearDatastore(ctx, cid.Cid{})
			if err != nil {
				return err
			}

			err = signalAndWaitForAll("run-complete-" + runID)
			if err != nil {
				return err
			}

			runenv.RecordMessage("Closing node")
			err = t.close()
			if err != nil {
				return err
			}
			// err = bsnode.Close()
			// if err != nil {
			// 	return err
			// }
		}
	}

	runenv.RecordMessage("Ending testcase")
	return nil
}
