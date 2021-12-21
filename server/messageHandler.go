package server

import (
	"encoding/json"
	"fmt"
	"sync"

	"example_poh.com/client"
	"example_poh.com/config"
	"example_poh.com/dataType"
	"example_poh.com/poh"
)

type MessageHandler struct {
	StartPOHChan         chan bool
	ValidatorConnections map[string]*client.Client // map address to validator connection
	mu                   sync.Mutex
	OnlineValidator      map[string]*client.Client
	//POH relates
	ReceiveLeaderTickChan     chan poh.POHTick
	ReceiveVotedBlockChan     chan poh.POHBlock
	ReceiveValidatorVotesChan chan poh.POHVote
}

func (handler *MessageHandler) ProcessMessage(message dataType.Message) {
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

func (handler *MessageHandler) handlerValidatorStarted(message dataType.Message) {
	handler.mu.Lock()
	defer handler.mu.Unlock()
	// add to online
	handler.OnlineValidator[message.Header.From] = handler.ValidatorConnections[message.Header.From]
	fmt.Printf("Total validator started %v", len(handler.OnlineValidator))
	if len(handler.OnlineValidator) == len(config.AppConfig.Validators) {
		// runPoh()
		handler.StartPOHChan <- true
	}
	// send started back
	if message.Header.Type == "request" {
		handler.ValidatorConnections[message.Header.From].SendStarted("response")
	}
}

func (handler *MessageHandler) convertBodyToStruct(body interface{}, str interface{}) {
	jsonBody, _ := json.Marshal(body)
	json.Unmarshal(jsonBody, str)
}

func (handler *MessageHandler) handlerLeaderTick(message dataType.Message) {
	var tick poh.POHTick
	json.Unmarshal([]byte(message.Body), &tick)
	handler.ReceiveLeaderTickChan <- tick
}

func (handler *MessageHandler) handlerVotedBlock(message dataType.Message) {
	var block poh.POHBlock
	json.Unmarshal([]byte(message.Body), &block)
	handler.ReceiveVotedBlockChan <- block
}

func (handler *MessageHandler) handlerVoteLeaderBlock(message dataType.Message) {
	var vote poh.POHVote
	json.Unmarshal([]byte(message.Body), &vote)
	handler.ReceiveValidatorVotesChan <- vote
}
