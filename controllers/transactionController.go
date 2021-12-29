package controllers

import (
	"encoding/hex"

	pb "example_poh.com/proto"
	"github.com/ethereum/go-ethereum/crypto"
	"google.golang.org/protobuf/proto"
)

func GetTransactionHash(transaction *pb.Transaction) string {
	hashData := &pb.HashData{
		Address:      transaction.Address,
		Pubkey:       transaction.Pubkey,
		Data:         transaction.Data,
		PreviousHash: transaction.PreviousData.Hash,
		Balance:      transaction.Balance,
		Link:         transaction.Link,
	}
	b, _ := proto.Marshal(hashData)
	hash := crypto.Keccak256(b)
	transaction.Hash = hex.EncodeToString(hash)
	return transaction.Hash
}
