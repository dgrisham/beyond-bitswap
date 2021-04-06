package test

var exchangesTrade [][]uint64

func getBytesSentTrade(senderIndex int, recvIndex int) uint64 {
	return exchangesTrade[senderIndex][recvIndex]
}

func setBytesSentTrade(senderIndex int, recvIndex int, bytes uint64) {
	if exchangesTrade[senderIndex][recvIndex] < bytes {
		exchangesTrade[senderIndex][recvIndex] = bytes
	}
}

func copyLedgerData(data [][]uint64) [][]uint64 {
	exchangesTrade = make([][]uint64, len(data))
	for i, row := range data {
		exchangesTrade[i] = make([]uint64, len(row))
		for j, entry := range row {
			exchangesTrade[i][j] = entry
		}
	}
	return exchangesTrade
}
