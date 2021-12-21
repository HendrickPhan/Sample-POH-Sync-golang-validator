package poh

func (block *POHBlock) GetTotalTransaction() int {
	totalTransaction := 0
	for _, tick := range block.Ticks {
		for _, hash := range tick.Hashes {
			totalTransaction += len(hash.Transactions)
		}
	}

	return totalTransaction
}
