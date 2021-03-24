package test

import "github.com/protocol/beyond-bitswap/testbed/testbed/utils"

var initialSends = map[utils.NodeType]map[int]map[utils.NodeType]map[int]uint64{
	utils.Leech: map[int]map[utils.NodeType]map[int]uint64{
		0: map[utils.NodeType]map[int]uint64{
			utils.Seed: map[int]uint64{
				0: 1,
			},
		},
		1: map[utils.NodeType]map[int]uint64{
			utils.Seed: map[int]uint64{
				0: 25000,
			},
		},
		2: map[utils.NodeType]map[int]uint64{
			utils.Seed: map[int]uint64{
				0: 500000,
			},
		},
	},
	utils.Seed: map[int]map[utils.NodeType]map[int]uint64{
		0: map[utils.NodeType]map[int]uint64{
			utils.Leech: map[int]uint64{
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
