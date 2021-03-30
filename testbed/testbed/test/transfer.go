package test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"

	"github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/protocol/beyond-bitswap/testbed/testbed/utils"
)

// Transfer data from S seeds to L leeches
func Transfer(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
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

	// For each test permutation found in the test
	for pIndex, testParams := range testvars.Permutations {
		// Set up network (with traffic shaping)
		if err := utils.SetupNetwork(ctx, runenv, t.nwClient, t.nodetp, t.tpindex, testParams.Latency,
			testParams.Bandwidth, testParams.JitterPct); err != nil {
			return fmt.Errorf("Failed to set up network: %v", err)
		}

		// Accounts for every file that couldn't be found.
		var leechFails int64
		var rootCid cid.Cid

		// Wait for all nodes to be ready to start the run
		err = signalAndWaitForAll(fmt.Sprintf("start-file-%d", pIndex))
		if err != nil {
			return err
		}

		switch t.nodetp {
		case utils.Seed:
			rootCid, err = t.addPublishFile(ctx, pIndex, testParams.File, runenv, testvars)
		case utils.Leech:
			rootCid, err = t.readFile(ctx, pIndex, runenv, testvars)
		}
		if err != nil {
			return err
		}

		runenv.RecordMessage("File injest complete...")
		// Wait for all nodes to be ready to dial
		err = signalAndWaitForAll(fmt.Sprintf("injest-complete-%d", pIndex))
		if err != nil {
			return err
		}

		if testvars.TCPEnabled {
			runenv.RecordMessage("Running TCP test...")
			switch t.nodetp {
			case utils.Seed:
				err = t.runTCPServer(ctx, pIndex, 0, testParams.File, runenv, testvars)
			case utils.Leech:
				tcpFetch, err = t.runTCPFetch(ctx, pIndex, 0, runenv, testvars)
			}
			if err != nil {
				return err
			}
		}

		runenv.RecordMessage("Starting %s Fetch...", nodeType)

		for runNum := 1; runNum < testvars.RunCount+1; runNum++ {
			// Reset the timeout for each run
			ctx, cancel := context.WithTimeout(ctx, testvars.RunTimeout)
			defer cancel()

			runID := fmt.Sprintf("%d-%d", pIndex, runNum)

			// Wait for all nodes to be ready to start the run
			err = signalAndWaitForAll("start-run-" + runID)
			if err != nil {
				return err
			}

			runenv.RecordMessage("Starting run %d / %d (%d bytes)", runNum, testvars.RunCount, testParams.File.Size())

			// @dgrisham: all seeders and leechers are connected, but no two nodes of the same type connect to each
			// other (so no seed <-> seed or leech <-> leech connections)
			var peersToDial []utils.PeerInfo
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

				numBytesSent := getInitialSend(t.nodetp, t.tpindex, peerInfo.Nodetp, peerInfo.TpIndex)
				if numBytesSent != 0 {
					runenv.RecordMessage("Setting sent value in ledger to %d bytes for %s %d (peer %s)", numBytesSent, peerInfo.Nodetp, peerInfo.TpIndex, peerInfo.Addr.ID.String())
					bsnode.Bitswap.SetLedgerSentBytes(peerInfo.Addr.ID, int(numBytesSent))
				}

				numBytesRcvd := getInitialSend(peerInfo.Nodetp, peerInfo.TpIndex, t.nodetp, t.tpindex)
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
							setSend(t.nodetp, t.tpindex, peerInfo.Nodetp, peerInfo.TpIndex, receipt.Sent)
							setSend(peerInfo.Nodetp, peerInfo.TpIndex, t.nodetp, t.tpindex, receipt.Sent)
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
				// For each wave
				for waveNum := 0; waveNum < testvars.NumWaves; waveNum++ {
					// Only leecheers for that wave entitled to leech.
					if (t.tpindex % testvars.NumWaves) == waveNum {
						runenv.RecordMessage("Starting wave %d", waveNum)
						// Stagger the start of the first request from each leech
						// Note: seq starts from 1 (not 0)
						startDelay := time.Duration(t.seq-1) * testvars.RequestStagger

						runenv.RecordMessage("Starting to leech %d / %d (%d bytes)", runNum, testvars.RunCount, testParams.File.Size())
						runenv.RecordMessage("Leech fetching data after %s delay", startDelay)
						start := time.Now()
						// TODO: Here we may be able to define requesting pattern. ipfs.DAG()
						// Right now using a path.
						ctxFetch, cancel := context.WithTimeout(ctx, testvars.RunTimeout/2)
						// Pin Add also traverse the whole DAG
						// err := ipfsNode.API.Pin().Add(ctxFetch, fPath)
						rcvFile, err := transferNode.Fetch(ctxFetch, rootCid, t.peerInfos)
						if err != nil {
							runenv.RecordMessage("Error fetching data: %v", err)
							leechFails++
						} else {
							runenv.RecordMessage("Fetch complete, proceeding")
							err = files.WriteTo(rcvFile, "/tmp/"+strconv.Itoa(t.tpindex)+time.Now().String())
							if err != nil {
								cancel()
								return err
							}
							timeToFetch = time.Since(start)
							s, _ := rcvFile.Size()
							runenv.RecordMessage("Leech fetch of %d complete (%d ns) for wave %d", s, timeToFetch, waveNum)
						}
						cancel()
					}
					if waveNum < testvars.NumWaves-1 {
						runenv.RecordMessage("Waiting 5 seconds between waves for wave %d", waveNum)
						time.Sleep(5 * time.Second)
					}
					_, err = t.client.SignalAndWait(ctx, sync.State(fmt.Sprintf("leech-wave-%d", waveNum)), testvars.LeechCount)
				}
			}

			// Wait for all leeches to have downloaded the data from seeds
			err = signalAndWaitForAll("transfer-complete-" + runID)
			if err != nil {
				return err
			}

			/// --- Report stats
			err = t.emitMetrics(runenv, runNum, nodeType, testParams, timeToFetch, tcpFetch, leechFails, testvars.MaxConnectionRate)
			if err != nil {
				return err
			}
			runenv.RecordMessage("Finishing emitting metrics. Starting to clean...")

			err = t.cleanupRun(ctx, rootCid, runenv)
			if err != nil {
				return err
			}
		}
		err = t.cleanupFile(ctx, rootCid)
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

type nodeInitializer func(ctx context.Context, runenv *runtime.RunEnv, testvars *TestVars, baseT *TestData) (*NodeTestData, error)

var supportedNodes = map[string]nodeInitializer{
	"ipfs":       initializeIPFSTest,
	"bitswap":    initializeBitswapTest,
	"graphsync":  initializeGraphsyncTest,
	"libp2pHTTP": initializeLibp2pHTTPTest,
	"rawLibp2p":  initializeRawLibp2pTest,
	// TODO FIX HTTP
	//"http":       initializeHTTPTest,
}

func initializeIPFSTest(ctx context.Context, runenv *runtime.RunEnv, testvars *TestVars, baseT *TestData) (*NodeTestData, error) {
	// Create IPFS node
	runenv.RecordMessage("Preparing exchange for node: %v", testvars.ExchangeInterface)
	// Set exchange Interface
	exch, err := utils.SetExchange(ctx, testvars.ExchangeInterface)
	if err != nil {
		return nil, err
	}
	ipfsNode, err := utils.CreateIPFSNodeWithConfig(ctx, baseT.nConfig, exch, testvars.DHTEnabled, testvars.ProvidingEnabled)
	if err != nil {
		runenv.RecordFailure(err)
		return nil, err
	}

	err = baseT.signalAndWaitForAll("file-list-ready")
	if err != nil {
		return nil, err
	}

	return &NodeTestData{
		TestData: baseT,
		node:     ipfsNode,
	}, nil
}

func initializeBitswapTest(ctx context.Context, runenv *runtime.RunEnv, testvars *TestVars, baseT *TestData) (*NodeTestData, error) {
	h, err := makeHost(ctx, baseT)
	if err != nil {
		return nil, err
	}
	runenv.RecordMessage("I am %s with addrs: %v", h.ID(), h.Addrs())

	// Use the same blockstore on all runs for the seed node
	bstoreDelay := time.Duration(runenv.IntParam("bstore_delay_ms")) * time.Millisecond

	dStore, err := utils.CreateDatastore(testvars.DiskStore, bstoreDelay)
	if err != nil {
		return nil, err
	}
	runenv.RecordMessage("created data store %T with params disk_store=%b", dStore, testvars.DiskStore)
	bstore, err := utils.CreateBlockstore(ctx, dStore)
	if err != nil {
		return nil, err
	}
	// Create a new bitswap node from the blockstore
	bsnode, err := utils.CreateBitswapNode(ctx, h, bstore)
	if err != nil {
		return nil, err
	}

	return &NodeTestData{baseT, bsnode, &h}, nil
}

func initializeGraphsyncTest(ctx context.Context, runenv *runtime.RunEnv, testvars *TestVars, baseT *TestData) (*NodeTestData, error) {
	h, err := makeHost(ctx, baseT)
	if err != nil {
		return nil, err
	}
	runenv.RecordMessage("I am %s with addrs: %v", h.ID(), h.Addrs())

	// Use the same blockstore on all runs for the seed node
	bstoreDelay := time.Duration(runenv.IntParam("bstore_delay_ms")) * time.Millisecond
	dStore, err := utils.CreateDatastore(testvars.DiskStore, bstoreDelay)
	if err != nil {
		return nil, err
	}
	runenv.RecordMessage("created data store %T with params disk_store=%v", dStore, testvars.DiskStore)
	bstore, err := utils.CreateBlockstore(ctx, dStore)
	if err != nil {
		return nil, err
	}

	// Create a new bitswap node from the blockstore
	numSeeds := runenv.TestInstanceCount - (testvars.LeechCount + testvars.PassiveCount)
	bsnode, err := utils.CreateGraphsyncNode(ctx, h, bstore, numSeeds)
	if err != nil {
		return nil, err
	}

	return &NodeTestData{baseT, bsnode, &h}, nil
}

func initializeLibp2pHTTPTest(ctx context.Context, runenv *runtime.RunEnv, testvars *TestVars, baseT *TestData) (*NodeTestData, error) {
	if runenv.TestInstanceCount != 2 {
		return nil, errors.New("libp2p HTTP transfer ONLY supports two instances for now")
	}

	if testvars.LeechCount != 1 {
		return nil, errors.New("libp2p HTTP transfer ONLY supports 1 Leecher for now")
	}

	if testvars.PassiveCount != 0 {
		return nil, errors.New("libp2p HTTP transfer does NOT support passive peers")
	}

	h, err := makeHost(ctx, baseT)
	if err != nil {
		return nil, err
	}
	runenv.RecordMessage("I am %s with addrs: %v", h.ID(), h.Addrs())

	libp2pHttpN, err := utils.CreateLibp2pHTTPNode(ctx, h, baseT.nodetp)
	if err != nil {
		return nil, err
	}

	return &NodeTestData{
		TestData: baseT,
		node:     libp2pHttpN,
		host:     &h,
	}, nil
}

func initializeHTTPTest(ctx context.Context, runenv *runtime.RunEnv, testvars *TestVars, baseT *TestData) (*NodeTestData, error) {
	if runenv.TestInstanceCount != 2 {
		return nil, errors.New("http transfer ONLY supports two instances for now")
	}

	if testvars.LeechCount != 1 {
		return nil, errors.New("http transfer ONLY supports 1 Leecher for now")
	}

	if testvars.PassiveCount != 0 {
		return nil, errors.New("http transfer does NOT support passive peers")
	}

	h, err := makeHost(ctx, baseT)
	if err != nil {
		return nil, err
	}
	runenv.RecordMessage("I am %s with addrs: %v", h.ID(), h.Addrs())

	httpN, err := utils.CreateHTTPNode(ctx, h, baseT.nodetp)
	if err != nil {
		return nil, err
	}

	return &NodeTestData{
		TestData: baseT,
		node:     httpN,
		host:     &h,
	}, nil
}

func initializeRawLibp2pTest(ctx context.Context, runenv *runtime.RunEnv, testvars *TestVars, baseT *TestData) (*NodeTestData, error) {
	if runenv.TestInstanceCount != 2 {
		return nil, errors.New("libp2p transfer ONLY supports two instances for now")
	}

	if testvars.LeechCount != 1 {
		return nil, errors.New("libp2p transfer ONLY supports 1 Leecher for now")
	}

	if testvars.PassiveCount != 0 {
		return nil, errors.New("libp2P transfer does NOT support passive peers")
	}

	h, err := makeHost(ctx, baseT)
	if err != nil {
		return nil, err
	}
	runenv.RecordMessage("I am %s with addrs: %v", h.ID(), h.Addrs())

	rawLibp2pN, err := utils.CreateRawLibp2pNode(ctx, h, baseT.nodetp)
	if err != nil {
		return nil, err
	}

	return &NodeTestData{
		TestData: baseT,
		node:     rawLibp2pN,
		host:     &h,
	}, nil
}

func makeHost(ctx context.Context, baseT *TestData) (host.Host, error) {
	// Create libp2p node
	privKey, err := crypto.UnmarshalPrivateKey(baseT.nConfig.PrivKey)
	if err != nil {
		return nil, err
	}

	return libp2p.New(ctx, libp2p.Identity(privKey), libp2p.ListenAddrs(baseT.nConfig.AddrInfo.Addrs...))
}
