package server

import (
	"sort"
	"sync"

	"example_poh.com/client"
	"example_poh.com/config"
	"example_poh.com/dataType"
	pb "example_poh.com/proto"
	"google.golang.org/protobuf/proto"
)

type MessageHandler struct {
	StartPOHChan         chan bool
	Validators           []dataType.Validator // list validator in right order to find leader
	LeaderIndex          int
	ValidatorConnections map[string]*client.Client // map address to validator connection
	mu                   sync.Mutex
	OnlineValidator      map[string]*client.Client
	//POH relates
	ReceiveLeaderTickChan   chan *pb.POHTick
	ReceiveVotedBlockChan   chan *pb.POHBlock
	ReceiveVoteChan         chan *pb.POHVote
	LeaderIndexChan         chan int
	ReceiveCheckedBlockChan chan *pb.CheckedBlock
}

func (handler *MessageHandler) AddValidators(validators []dataType.Validator) {
	sort.Slice(validators, func(i, j int) bool {
		return validators[i].Address < validators[j].Address
	})
	handler.Validators = validators
}

func (handler *MessageHandler) ProcessMessage(message *pb.Message) {
	switch message.Header.Command {
	case "ValidatorStarted":
		handler.handlerValidatorStarted(message)
	case "LeaderTick":
		handler.handlerLeaderTick(message)
	case "VotedBlock":
		handler.handlerVotedBlock(message)
	case "VoteLeaderBlock":
		handler.handlerVoteLeaderBlock(message)
	case "SendCheckedBlock":
		handler.handlerSendCheckedBlock(message)

	}
}

func (handler *MessageHandler) handlerValidatorStarted(message *pb.Message) {
	handler.mu.Lock()
	defer handler.mu.Unlock()
	// add to online
	handler.OnlineValidator[message.Header.From] = handler.ValidatorConnections[message.Header.From]
	if len(handler.OnlineValidator) == len(config.AppConfig.Validators) {
		// runPoh()
		go func() {
			handler.StartPOHChan <- true
		}()
	}
	// send started back
	if message.Header.Type == "request" {
		handler.ValidatorConnections[message.Header.From].SendStarted("response")
	}
}

func (handler *MessageHandler) handlerLeaderTick(message *pb.Message) {
	if handler.Validators[handler.LeaderIndex].Address != message.Header.From {
		// fmt.Printf("ERR: Tick not from leader")
		return // tick must from current leader or this node will skip it
	}
	tick := &pb.POHTick{}
	proto.Unmarshal([]byte(message.Body), tick)
	handler.ReceiveLeaderTickChan <- tick
}

func (handler *MessageHandler) handlerVotedBlock(message *pb.Message) {
	if handler.Validators[handler.LeaderIndex].Address != message.Header.From {
		return // voted block must from current leader or this node will skip it
	}
	block := &pb.POHBlock{}
	proto.Unmarshal([]byte(message.Body), block)
	handler.ReceiveVotedBlockChan <- block
}

func (handler *MessageHandler) handlerVoteLeaderBlock(message *pb.Message) {
	vote := &pb.POHVote{}
	proto.Unmarshal([]byte(message.Body), vote)
	handler.ReceiveVoteChan <- vote
}

func (handler *MessageHandler) handlerSendCheckedBlock(message *pb.Message) {
	var nextLeaderIdx int
	if handler.LeaderIndex+1 == len(handler.Validators) { // end of validator list so return to first validator
		nextLeaderIdx = 0
	} else {
		nextLeaderIdx = handler.LeaderIndex + 1
	}
	nextLeader := handler.Validators[nextLeaderIdx]
	if nextLeader.Address == config.AppConfig.Address { // this node is next leader
		// send transaction to recorder
		checkedBlock := &pb.CheckedBlock{}
		proto.Unmarshal([]byte(message.Body), checkedBlock)
		handler.ReceiveCheckedBlockChan <- checkedBlock
	} else {
		// forward checked block to next leader
		nextLeaderClient := handler.ValidatorConnections[nextLeader.Address]
		nextLeaderClient.SendMessage(message)
	}
}
