package controllers

import (
	"example_poh.com/config"
	pb "example_poh.com/proto"
)

func GenerateGenesisBlock(genesisBlockInfo config.GenesisBlockInfo) *pb.POHBlock {
	genesisTransaction := &pb.Transaction{
		Address: genesisBlockInfo.InitAddress,
		Pubkey:  genesisBlockInfo.InitPubkey,
		Data:    "",
		PreviousData: &pb.Transaction{
			Hash: "GENESIS_HASH",
		},
		Balance: genesisBlockInfo.InitBalance,
		Link:    "",
		Sign:    "",
	}
	GetTransactionHash(genesisTransaction)

	lastHash := &pb.POHHash{
		Count:        1,
		LastHash:     "",
		Hash:         "INIT_HASH",
		Transactions: []*pb.Transaction{genesisTransaction},
	}
	lastTick := &pb.POHTick{
		Hashes: []*pb.POHHash{lastHash},
		Count:  1,
	}

	genesisblock := &pb.POHBlock{
		Ticks: []*pb.POHTick{lastTick},
		Count: 1,
		Type:  "leader",
		Hash:  lastHash,
	}
	return genesisblock
}
