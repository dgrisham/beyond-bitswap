package test

import "github.com/protocol/beyond-bitswap/testbed/testbed/utils"

var initialSends = map[utils.NodeType]map[int]map[utils.NodeType]map[int]int{
	utils.Leech: map[int]map[utils.NodeType]map[int]int{
		0: map[utils.NodeType]map[int]int{
			utils.Seed: map[int]int{
				0: 1,
			},
		},
		1: map[utils.NodeType]map[int]int{
			utils.Seed: map[int]int{
				0: 5000000000,
			},
		},
		2: map[utils.NodeType]map[int]int{
			utils.Seed: map[int]int{
				0: 5000000000,
			},
		},
	},
	utils.Seed: map[int]map[utils.NodeType]map[int]int{
		0: map[utils.NodeType]map[int]int{
			utils.Leech: map[int]int{
				0: 1000,
				1: 1000,
				2: 1000,
			},
		},
	},
}

func getInitialSend(senderType utils.NodeType, senderIndex int, recvType utils.NodeType, recvIndex int) int {
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
