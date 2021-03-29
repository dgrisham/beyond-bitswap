package test

import (
	"github.com/protocol/beyond-bitswap/testbed/testbed/utils"
	"github.com/testground/sdk-go/runtime"
)

type Exchanges map[utils.NodeType]map[int]map[utils.NodeType]map[int]uint64

var exchanges Exchanges

func makeInitialSends(runenv *runtime.RunEnv, ratios []float64) Exchanges {
	initialSends := Exchanges{
		utils.Seed: {
			0: {
				utils.Leech: {
					0: 200000000,
					1: 200000000,
					2: 1000,
				},
			},
		},
		utils.Leech: {},
	}

	for i, ratio := range ratios {
		initialSends[utils.Leech][i] = map[utils.NodeType]map[int]uint64{
			utils.Seed: {
				0: uint64(ratio * float64(initialSends[utils.Seed][0][utils.Leech][i])),
			},
		}
	}

	return initialSends
}

func getBytesSent(senderType utils.NodeType, senderIndex int, recvType utils.NodeType, recvIndex int) uint64 {
	if _, ok := exchanges[senderType]; !ok {
		return 0
	}
	if _, ok := exchanges[senderType][senderIndex]; !ok {
		return 0
	}
	if _, ok := exchanges[senderType][senderIndex][recvType]; !ok {
		return 0
	}
	if _, ok := exchanges[senderType][senderIndex][recvType][recvIndex]; !ok {
		return 0
	}

	return exchanges[senderType][senderIndex][recvType][recvIndex]
}

func setBytesSent(senderType utils.NodeType, senderIndex int, recvType utils.NodeType, recvIndex int, bytes uint64) {
	if _, ok := exchanges[senderType]; !ok {
		return
	}
	if _, ok := exchanges[senderType][senderIndex]; !ok {
		return
	}
	if _, ok := exchanges[senderType][senderIndex][recvType]; !ok {
		return
	}
	if _, ok := exchanges[senderType][senderIndex][recvType][recvIndex]; !ok {
		return
	}

	if exchanges[senderType][senderIndex][recvType][recvIndex] < bytes {
		exchanges[senderType][senderIndex][recvType][recvIndex] = bytes
	}
}

var initialSendsTrade = map[int]map[int]uint64{
	0: {
		0: 1,
	},
	1: {
		0: 25000,
	},
	2: {
		0: 500000,
	},
}

func getInitialSendTrade(senderIndex int, recvIndex int) uint64 {
	if _, ok := initialSendsTrade[senderIndex]; !ok {
		return 0
	}
	if _, ok := initialSendsTrade[senderIndex][recvIndex]; !ok {
		return 0
	}

	return initialSendsTrade[senderIndex][recvIndex]
}

func setSendTrade(senderIndex int, recvIndex int, bytes uint64) {
	if _, ok := initialSendsTrade[senderIndex]; !ok {
		return
	}
	if _, ok := initialSendsTrade[senderIndex][recvIndex]; !ok {
		return
	}

	if initialSendsTrade[senderIndex][recvIndex] < bytes {
		initialSendsTrade[senderIndex][recvIndex] = bytes
	}
}
