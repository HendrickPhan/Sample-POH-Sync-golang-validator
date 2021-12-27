package poh

import pb "example_poh.com/proto"

func GetTickHash(tick *pb.POHTick) string {
	lastHash := tick.Hashes[len(tick.Hashes)-1].Hash
	return lastHash
}
