package main

import (
	test "github.com/adlrocha/beyond-bitswap/testbed/test"
	"github.com/testground/sdk-go/run"
)

func main() {
	run.InvokeMap(map[string]interface{}{
		"transfer":      test.Transfer,
		"ipfs-transfer": test.IPFSTransfer,
	})
}