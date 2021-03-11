package test

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"

	"github.com/protocol/beyond-bitswap/testbed/testbed/utils"
	"github.com/protocol/beyond-bitswap/testbed/testbed/utils/dialer"
)

// Transfer data from S seeds to L leeches
func Transfer(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	// Test Parameters
	testvars := getEnvVars(runenv)
	bstoreDelay := time.Duration(runenv.IntParam("bstore_delay_ms")) * time.Millisecond
	fileSizes, err := utils.ParseIntArray(runenv.StringParam("file_size"))
	if err != nil {
		return err
	}
	roundSize := runenv.IntParam("round_size")
	strategyFunc := runenv.StringParam("strategy_func")

	runenv.RecordMessage("Running on branch 'peer-weights'")

	/// --- Set up
	ctx, cancel := context.WithTimeout(context.Background(), testvars.Timeout)
	defer cancel()
	t, err := InitializeTest(ctx, runenv, testvars)
	if err != nil {
		return err
	}
	// Create libp2p node
	privKey, err := crypto.UnmarshalPrivateKey(t.nConfig.PrivKey)
	if err != nil {
		return err
	}

	h, err := libp2p.New(ctx, libp2p.Identity(privKey), libp2p.ListenAddrs(t.nConfig.AddrInfo.Addrs...))
	if err != nil {
		return err
	}
	defer h.Close()
	runenv.RecordMessage("I am %s with addrs: %v", h.ID(), h.Addrs())

	// Use the same blockstore on all runs for the seed node
	var bstore blockstore.Blockstore
	var bsnode *utils.Node
	if t.nodetp == utils.Seed {
		bstore, err = utils.CreateBlockstore(ctx, bstoreDelay)
		if err != nil {
			return err
		}
		// Create a new bitswap node from the blockstore
		bsnode, err = utils.CreateBitswapNode(ctx, h, bstore, strategyFunc, roundSize)
		if err != nil {
			return err
		}
	}

	// Signal that this node is in the given state, and wait for all peers to
	// send the same signal
	signalAndWaitForAll := t.signalAndWaitForAll

	// For each file size
	for sizeIndex, fileSize := range fileSizes {
		// If the total amount of seed data to be generated is greater than
		// parallelGenMax, generate seed data in series
		// genSeedSerial := seedCount > 2 || fileSize*seedCount > parallelGenMax
		genSeedSerial := true

		// Run the test runCount times
		var rootCid cid.Cid

		// Wait for all nodes to be ready to start the run
		err = signalAndWaitForAll(fmt.Sprintf("start-file-%d", sizeIndex))
		if err != nil {
			return err
		}

		switch t.nodetp {
		case utils.Seed:
			seedGenerated := sync.State(fmt.Sprintf("seed-generated-%d", sizeIndex))
			var start time.Time
			if genSeedSerial {
				// Each seed generates the seed data in series, to avoid
				// overloading a single machine hosting multiple instances
				if t.seedIndex > 0 {
					// Wait for the seeds with an index lower than this one
					// to generate their seed data
					doneCh := t.client.MustBarrier(ctx, seedGenerated, int(t.seedIndex)).C
					if err = <-doneCh; err != nil {
						return err
					}
				}

				// Generate a file of the given size and add it to the datastore
				start = time.Now()
			}
			runenv.RecordMessage("Generating seed data of %d bytes", fileSize)

			rootCid, err := setupSeed(ctx, runenv, bsnode, fileSize, int(t.seedIndex))
			if err != nil {
				return fmt.Errorf("Failed to set up seed: %w", err)
			}

			if genSeedSerial {
				runenv.RecordMessage("Done generating seed data of %d bytes (%s)", fileSize, time.Since(start))

				// Signal we've completed generating the seed data
				_, err = t.client.SignalEntry(ctx, seedGenerated)
				if err != nil {
					return fmt.Errorf("Failed to signal seed generated: %w", err)
				}
			}
			err = t.publishFile(ctx, sizeIndex, &rootCid, runenv)
		case utils.Leech:
			rootCid, err = t.readFile(ctx, sizeIndex, runenv, testvars)
		}
		if err != nil {
			return err
		}

		runenv.RecordMessage("File injest complete...")
		// Wait for all nodes to be ready to dial
		err = signalAndWaitForAll(fmt.Sprintf("injest-complete-%d", sizeIndex))
		if err != nil {
			return err
		}

		for runNum := 1; runNum < testvars.RunCount+1; runNum++ {
			// Reset the timeout for each run
			ctx, cancel := context.WithTimeout(ctx, testvars.RunTimeout)
			defer cancel()

			runID := fmt.Sprintf("%d-%d", sizeIndex, runNum)

			// Wait for all nodes to be ready to start the run
			err = signalAndWaitForAll("start-run-" + runID)
			if err != nil {
				return err
			}

			runenv.RecordMessage("Starting run %d / %d (%d bytes)", runNum, testvars.RunCount, fileSize)

			if t.nodetp == utils.Leech {
				// For leeches, create a new blockstore on each run
				bstore, err = utils.CreateBlockstore(ctx, bstoreDelay)
				if err != nil {
					return err
				}

				// Create a new bitswap node from the blockstore
				bsnode, err = utils.CreateBitswapNode(ctx, h, bstore, strategyFunc, roundSize)
				if err != nil {
					return err
				}
			}

			// Wait for all nodes to be ready to dial
			err = signalAndWaitForAll("ready-to-connect-" + runID)
			if err != nil {
				return err
			}

			var peersToDial []dialer.PeerInfo
			switch t.nodetp {
			case utils.Seed:
				for _, peerInfo := range t.peerInfos {
					if peerInfo.Nodetp == utils.Leech {
						peersToDial = append(peersToDial, peerInfo)
					}
				}
			case utils.Leech:
				for _, peerInfo := range t.peerInfos {
					if peerInfo.Nodetp == utils.Seed {
						peersToDial = append(peersToDial, peerInfo)
					}
				}
			}

			dialed, err := t.dialFn(ctx, h, t.nodetp, peersToDial, testvars.MaxConnectionRate)
			if err != nil {
				return err
			}
			runenv.RecordMessage("Dialed %d other nodes", len(dialed))

			// Wait for all nodes to be connected
			err = signalAndWaitForAll("connect-complete-" + runID)
			if err != nil {
				return err
			}

			for _, peerInfo := range t.peerInfos {

				numBytesSent := getInitialSend(t.nodetp, t.tpindex, peerInfo.Nodetp, peerInfo.TpIndex)
				if numBytesSent != 0 {
					runenv.RecordMessage("Adding %d bytes to sent value in ledger for %s %d (peer %s)", numBytesSent, peerInfo.Nodetp, peerInfo.TpIndex, peerInfo.Addr.ID.String())
					bsnode.Bitswap.AddToLedgerSentBytes(peerInfo.Addr.ID, numBytesSent)
				}

				numBytesRcvd := getInitialSend(peerInfo.Nodetp, peerInfo.TpIndex, t.nodetp, t.tpindex)
				if numBytesRcvd != 0 {
					runenv.RecordMessage("Adding %d bytes to received value ledger for %s %d (peer %s)", numBytesRcvd, peerInfo.Nodetp, peerInfo.TpIndex, peerInfo.Addr.ID.String())
					bsnode.Bitswap.AddToLedgerReceivedBytes(peerInfo.Addr.ID, numBytesRcvd)
				}
			}

			// Wait for all nodes to initialize their ledgers
			err = signalAndWaitForAll("ledger-initialization-complete-" + runID)
			if err != nil {
				return err
			}

			// --- start time series metric gathering functions
			quit := make(chan bool)
			go func() { // record bitswap metrics in the background while fetching blocks

				for {
					select {

					case <-quit: // loop until signal is received
						return

					default:

						for _, peerInfo := range t.peerInfos {
							if peerInfo.Addr.ID == h.ID() {
								continue
							}
							receipt := bsnode.Bitswap.LedgerForPeer(peerInfo.Addr.ID)
							receiptID := fmt.Sprintf("receiptAtTime/peer:%s/sent:%v/recv:%v/value:%v/exchanged:%v/weight:%v/workRemaining:%v", receipt.Peer, receipt.Sent, receipt.Recv, receipt.Value, receipt.Exchanged, receipt.Weight, receipt.WorkRemaining)
							runenv.R().RecordPoint(receiptID, float64(1))
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

			var timeToFetch time.Duration
			if t.nodetp == utils.Leech {
				// Stagger the start of the first request from each leech
				// Note: seq starts from 1 (not 0)
				// startDelay := time.Duration(t.seq-1) * testvars.RequestStagger
				// time.Sleep(startDelay)

				// runenv.RecordMessage("Leech fetching data after %s delay", startDelay)
				start := time.Now()
				err := bsnode.FetchGraph(ctx, rootCid)
				timeToFetch = time.Since(start)
				if err != nil {
					return fmt.Errorf("Error fetching data through Bitswap: %w", err)
				}
				runenv.RecordMessage("Leech fetch complete (%s)", timeToFetch)
			}

			// Wait for all leeches to have downloaded the data from seeds
			err = signalAndWaitForAll("transfer-complete-" + runID)
			if err != nil {
				return err
			}

			/// --- Report stats
			err = emitMetrics(runenv, bsnode, runNum, t.seq, t.grpseq, t.latency, t.bandwidth, fileSize, t.nodetp, t.tpindex, timeToFetch)
			if err != nil {
				return err
			}

			// Shut down bitswap
			err = bsnode.Close()
			if err != nil {
				return fmt.Errorf("Error closing Bitswap: %w", err)
			}

			// Disconnect peers
			for _, c := range h.Network().Conns() {
				err := c.Close()
				if err != nil {
					return fmt.Errorf("Error disconnecting: %w", err)
				}
			}

			if t.nodetp == utils.Leech {
				// Free up memory by clearing the leech blockstore at the end of each run.
				// Note that although we create a new blockstore for the leech at the
				// start of the run, explicitly cleaning up the blockstore from the
				// previous run allows it to be GCed.
				if err := utils.ClearBlockstore(ctx, bstore); err != nil {
					return fmt.Errorf("Error clearing blockstore: %w", err)
				}
			}
		}
		if t.nodetp == utils.Seed {
			// Free up memory by clearing the seed blockstore at the end of each
			// set of tests over the current file size.
			if err := utils.ClearBlockstore(ctx, bstore); err != nil {
				return fmt.Errorf("Error clearing blockstore: %w", err)
			}
		}
	}

	/// --- Ending the test

	return nil
}
