package controllers

import (
	"encoding/hex"

	pb "example_poh.com/proto"
	"github.com/ethereum/go-ethereum/crypto"
	"google.golang.org/protobuf/proto"
)

func GetTransactionHash(transaction *pb.Transaction) string {
	hashData := &pb.HashData{
		FromAddress:  transaction.FromAddress,
		ToAddress:    transaction.ToAddress,
		Pubkey:       transaction.Pubkey,
		Data:         transaction.Data,
		PreviousHash: transaction.PreviousData.Hash,
		PendingUse:   transaction.PendingUse,
		Balance:      transaction.Balance,
	}
	b, _ := proto.Marshal(hashData)
	hash := crypto.Keccak256(b)
	transaction.Hash = hex.EncodeToString(hash)
	return transaction.Hash
}
