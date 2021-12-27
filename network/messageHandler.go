package network

import (
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"sync"

	"example_poh.com/config"
	"example_poh.com/dataType"
	pb "example_poh.com/proto"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

type MessageHandler struct {
	InitedConnectionsChan chan *Connection
	RemoveConnectionChan  chan *Connection
	ValidatorConnections  map[string]*Connection // map address to validator connection
	NodeConnections       map[string]*Connection // map address to validator connection

	StartPOHChan chan bool
	Validators   []dataType.Validator // list validator in right order to find leader
	LeaderIndex  int
	mu           sync.Mutex
	//POH relates
	ReceiveLeaderTickChan         chan *pb.POHTick
	ReceiveVoteChan               chan *pb.POHVote
	ReceiveVoteResultChan         chan *pb.POHVoteResult
	LeaderIndexChan               chan int
	ReceiveCheckedBlockChan       chan *pb.CheckedBlock
	ReceiveNextLeaderTickChan     chan *pb.POHTick
	ReceiveValidateTickResultChan chan *pb.POHValidateTickResult
}

func (handler *MessageHandler) OnConnect(conn *Connection) {
	log.Infof("OnConnect with server %s\n", conn.TCPConnection.RemoteAddr())
	conn.SendInitConnection()
}

func (handler *MessageHandler) OnDisconnect(conn *Connection) {
	log.Infof("Disconnected with server  %s, wallet address: %v \n", conn.TCPConnection.RemoteAddr(), conn.Address)
	handler.RemoveConnectionChan <- conn
	// TODO remove from connection list

}

func (handler *MessageHandler) HandleConnection(conn *Connection) {
	for {
		bLength := make([]byte, 8)
		_, err := conn.TCPConnection.Read(bLength)
		if err != nil {
			switch err {
			case io.EOF:
				handler.OnDisconnect(conn)
				return
			default:
				log.Error("server error: %v\n", err)
				return
			}
		}
		messageLength := uint64(binary.LittleEndian.Uint64(bLength))
		data := make([]byte, messageLength)
		byteRead, err := conn.TCPConnection.Read(data)
		if err != nil {
			switch err {
			case io.EOF:
				handler.OnDisconnect(conn)
				return
			default:
				log.Error("server error: %v\n", err)
				return
			}
		}
		if uint64(byteRead) != messageLength {
			log.Errorf("Invalid message receive byteRead !=  messageLength %v, %v\n", byteRead, messageLength)
		}

		message := pb.Message{}
		proto.Unmarshal(data[:messageLength], &message)
		go handler.ProcessMessage(conn, &message)
	}
}

func (handler *MessageHandler) ProcessMessage(conn *Connection, message *pb.Message) {
	// need init connection before do anything
	switch message.Header.Command {
	case "InitConnection":
		handler.handleInitConnectionMessage(conn, message)
	case "LeaderTick":
		handler.handleLeaderTick(message)
	case "VoteLeaderBlock":
		handler.handleVoteLeaderBlock(message)
	case "VoteResult":
		handler.handleVoteResult(message)
	case "SendCheckedBlock":
		handler.handleSendCheckedBlock(message)
	case "ValidateTickResult":
		handler.handleValidateTickResult(message)
	default:
	}
}

func (handler *MessageHandler) AddValidators(validators []dataType.Validator) {
	sort.Slice(validators, func(i, j int) bool {
		return validators[i].Address < validators[j].Address
	})
	handler.Validators = validators
}

func (handler *MessageHandler) UpdateLeaderIdx() {
	for {
		leaderIdx := <-handler.LeaderIndexChan
		handler.mu.Lock()
		handler.LeaderIndex = leaderIdx
		handler.mu.Unlock()
	}
}

func (handler *MessageHandler) handleInitConnectionMessage(conn *Connection, message *pb.Message) {
	initConnectionMessage := &pb.InitConnection{}
	proto.Unmarshal([]byte(message.Body), initConnectionMessage)
	conn.Address = initConnectionMessage.Address
	conn.Type = initConnectionMessage.Type
	handler.mu.Lock()
	if conn.Type == "Validator" {
		// TODO: should have node type in init connection to add connect to right list ex: validator, node, miner
		handler.ValidatorConnections[conn.Address] = conn
		if len(handler.ValidatorConnections) == len(config.AppConfig.Validators) {
			handler.StartPOHChan <- true

		}
	}
	if conn.Type == "Node" {
		handler.NodeConnections[conn.Address] = conn
	}

	log.Infof("Receive InitConnection from %v type %v\n", conn.TCPConnection.RemoteAddr(), conn.Type)
	go func() {
		handler.InitedConnectionsChan <- conn
	}()

	handler.mu.Unlock()
}

func (handler *MessageHandler) getNextLeaderIdx() int {
	nextLeaderIdx := handler.LeaderIndex + 1
	if nextLeaderIdx > len(handler.Validators)-1 {
		nextLeaderIdx = 0
	}
	return nextLeaderIdx
}

func (handler *MessageHandler) handleLeaderTick(message *pb.Message) {
	// forward tick to childs to validate
	for _, v := range handler.NodeConnections {
		v.SendMessage(message)
	}
}

func (handler *MessageHandler) handleValidateTickResult(message *pb.Message) {
	validateResult := &pb.POHValidateTickResult{}
	proto.Unmarshal([]byte(message.Body), validateResult)
	handler.ReceiveValidateTickResultChan <- validateResult
}

func (handler *MessageHandler) handleVoteResult(message *pb.Message) {
	if handler.Validators[handler.LeaderIndex].Address != message.Header.From {
		fmt.Printf("ERR: Vote result not from leader %v, %v\n", handler.Validators[handler.LeaderIndex].Address, message.Header.From)
		return // tick must from current leader or this node will skip it
	}
	voteResult := &pb.POHVoteResult{}
	proto.Unmarshal([]byte(message.Body), voteResult)
	handler.ReceiveVoteResultChan <- voteResult
}

func (handler *MessageHandler) handleVoteLeaderBlock(message *pb.Message) {
	vote := &pb.POHVote{}
	proto.Unmarshal([]byte(message.Body), vote)
	handler.ReceiveVoteChan <- vote
}

func (handler *MessageHandler) handleSendCheckedBlock(message *pb.Message) {
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
		if nextLeaderClient != nil {
			nextLeaderClient.SendMessage(message)
		}
	}
}
