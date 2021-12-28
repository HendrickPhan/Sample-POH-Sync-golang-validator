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
	Connections *Connections
	LeaderIndex int
	Validators  []dataType.Validator // list validator in right order to find leader
	mu          sync.Mutex

	//POH relates
	ReceiveLeaderTickChan         chan *pb.POHTick
	ReceiveVoteChan               chan *pb.POHVote
	ReceiveVoteResultChan         chan *pb.POHVoteResult
	ReceiveCheckedBlockChan       chan *pb.CheckedBlock
	ReceiveValidateTickResultChan chan *pb.POHValidateTickResult
	LeaderIndexChan               chan int
}

func (handler *MessageHandler) OnConnect(conn *Connection) {
	log.Infof("OnConnect with server %s\n", conn.TCPConnection.RemoteAddr())
	conn.SendInitConnection()
}

func (handler *MessageHandler) OnDisconnect(conn *Connection) {
	log.Infof("Disconnected with server  %s, wallet address: %v \n", conn.TCPConnection.RemoteAddr(), conn.Address)
	// TODO remove from connections

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

		for uint64(byteRead) != messageLength {
			log.Errorf("Invalid message receive byteRead !=  messageLength %v, %v\n", byteRead, messageLength)
			appendData := make([]byte, messageLength-uint64(byteRead))
			conn.TCPConnection.Read(appendData)

			data = append(data[:byteRead], appendData...)
			byteRead = len(data)
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

	handler.Connections.mu.Lock()
	if conn.Type == "Validator" {
		handler.Connections.ValidatorConnections[conn.Address] = conn
	}
	if conn.Type == "Node" {
		handler.Connections.NodeConnections[conn.Address] = conn
	}
	handler.Connections.mu.Unlock()
	log.Infof("Receive InitConnection from %v type %v\n", conn.TCPConnection.RemoteAddr(), conn.Type)
}

func (handler *MessageHandler) handleLeaderTick(message *pb.Message) {
	for _, v := range handler.Connections.NodeConnections {
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
		nextLeaderClient := handler.Connections.ValidatorConnections[nextLeader.Address]
		if nextLeaderClient != nil {
			nextLeaderClient.SendMessage(message)
		}
	}
}
