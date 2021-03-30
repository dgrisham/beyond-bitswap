package test

import "github.com/protocol/beyond-bitswap/testbed/testbed/utils"

var initialSends = map[utils.NodeType]map[int]map[utils.NodeType]map[int]uint64{
	utils.Leech: map[int]map[utils.NodeType]map[int]uint64{
		0: {
			utils.Seed: {
				0: 1,
			},
		},
		1: {
			utils.Seed: {
				0: 25000,
			},
		},
		2: {
			utils.Seed: {
				0: 500000,
			},
		},
	},
	utils.Seed: {
		0: {
			utils.Leech: {
				0: 1000,
				1: 1000,
				2: 1000,
			},
		},
	},
}

func getInitialSend(senderType utils.NodeType, senderIndex int, recvType utils.NodeType, recvIndex int) uint64 {
	if _, ok := initialSends[senderType]; !ok {
		return 0
	}
	if _, ok := initialSends[senderType][senderIndex]; !ok {
		return 0
	}
	if _, ok := initialSends[senderType][senderIndex][recvType]; !ok {
		return 0
	}
	if _, ok := initialSends[senderType][senderIndex][recvType][recvIndex]; !ok {
		return 0
	}

	return initialSends[senderType][senderIndex][recvType][recvIndex]
}

func setSend(senderType utils.NodeType, senderIndex int, recvType utils.NodeType, recvIndex int, bytes uint64) {
	if _, ok := initialSends[senderType]; !ok {
		return
	}
	if _, ok := initialSends[senderType][senderIndex]; !ok {
		return
	}
	if _, ok := initialSends[senderType][senderIndex][recvType]; !ok {
		return
	}
	if _, ok := initialSends[senderType][senderIndex][recvType][recvIndex]; !ok {
		return
	}

	if initialSends[senderType][senderIndex][recvType][recvIndex] < bytes {
		initialSends[senderType][senderIndex][recvType][recvIndex] = bytes
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
