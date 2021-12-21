package server

import (
	"fmt"
	"sync"

	"example_poh.com/client"
	"example_poh.com/config"
	pb "example_poh.com/proto"
	"google.golang.org/protobuf/proto"
)

type MessageHandler struct {
	StartPOHChan         chan bool
	ValidatorConnections map[string]*client.Client // map address to validator connection
	mu                   sync.Mutex
	OnlineValidator      map[string]*client.Client
	//POH relates
	ReceiveLeaderTickChan     chan *pb.POHTick
	ReceiveVotedBlockChan     chan *pb.POHBlock
	ReceiveValidatorVotesChan chan *pb.POHVote
	LeaderIndexChan           chan int
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
	}
}

func (handler *MessageHandler) handlerValidatorStarted(message *pb.Message) {
	handler.mu.Lock()
	defer handler.mu.Unlock()
	// add to online
	handler.OnlineValidator[message.Header.From] = handler.ValidatorConnections[message.Header.From]
	fmt.Printf("Total validator started %v", len(handler.OnlineValidator))
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
	tick := &pb.POHTick{}
	proto.Unmarshal([]byte(message.Body), tick)
	handler.ReceiveLeaderTickChan <- tick
}

func (handler *MessageHandler) handlerVotedBlock(message *pb.Message) {
	block := &pb.POHBlock{}
	proto.Unmarshal([]byte(message.Body), block)
	handler.ReceiveVotedBlockChan <- block
}

func (handler *MessageHandler) handlerVoteLeaderBlock(message *pb.Message) {
	vote := &pb.POHVote{}
	proto.Unmarshal([]byte(message.Body), vote)
	handler.ReceiveValidatorVotesChan <- vote
}
