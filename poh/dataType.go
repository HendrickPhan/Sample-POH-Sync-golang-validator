package poh

import (
	"sync"

	"example_poh.com/dataType"
	"example_poh.com/network"
	pb "example_poh.com/proto"
)

type POHBranch struct {
	Blocks           []*pb.POHBlock `json:"blocks"`
	TotalTransaction int            `json:"total_transaction"`
	TotalLeaderBlock int            `json:"total_leader_block"`
}

type POHRecorder struct {
	Transactions    []*pb.Transaction // pending transactions
	MainBranchIdx   int               // index of main branch
	StartBlockCount int64             // indicate from which block count this node will process, else remove
	Branches        []POHBranch       // save forks
}

type POHService struct {
	mu            sync.Mutex
	Server        *network.Server
	Checkpoint    *pb.POHBlock         // where new tick will start hash
	HashStack     []*pb.POHHash        // hash stack to create tick
	Recorder      POHRecorder          //
	Validators    []dataType.Validator // list validator to communicate, choose leader
	LeaderIndex   int                  // idx of leader in Validators
	HashPerSecond int                  //
	TickPerSecond int                  //
	TickPerSlot   int                  // these 3 variable use to sync hashrate between validators
	TimeOutTicks  int
	TickStart     int64 //
	TickEnd       int64 // use to throttle tick speed

	BlockChan             chan *pb.POHBlock
	ReceiveLeaderTickChan chan *pb.POHTick
	ReceiveVoteChan       chan *pb.POHVote
	ReceiveVoteResultChan chan *pb.POHVoteResult
	LeaderIndexChan       chan int

	ReceiveCheckedBlockChan chan *pb.CheckedBlock

	NextLeaderTicks           []*pb.POHTick
	ReceiveNextLeaderTickChan <-chan *pb.POHTick
}
