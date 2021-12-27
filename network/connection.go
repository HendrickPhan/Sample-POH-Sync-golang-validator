package network

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"

	"example_poh.com/config"
	pb "example_poh.com/proto"
	"google.golang.org/protobuf/proto"
)

type Connection struct {
	Address       string
	IP            string
	Port          int
	TCPConnection net.Conn
	Type          string
	mu            sync.Mutex
}

func (conn *Connection) SendMessage(message *pb.Message) error {
	b, err := proto.Marshal(message)
	if err != nil {
		fmt.Printf("Error when marshal %v", err)
		return err
	}
	length := make([]byte, 8)
	binary.LittleEndian.PutUint64(length, uint64(len(b)))
	conn.mu.Lock()
	conn.TCPConnection.Write(length)
	conn.TCPConnection.Write(b)
	conn.mu.Unlock()

	return nil
}

func (conn *Connection) SendInitConnection() {
	protoRs, _ := proto.Marshal(&pb.InitConnection{
		Address: config.AppConfig.Address,
		Type:    config.AppConfig.NodeType,
	})
	message := &pb.Message{
		Header: &pb.Header{
			Type:    "request",
			From:    config.AppConfig.Address,
			Command: "InitConnection",
		},
		Body: protoRs,
	}

	err := conn.SendMessage(message)
	if err != nil {
		fmt.Printf("Error when send started %v", err)
	}
}

func (conn *Connection) SendStarted(t string) {
	message := &pb.Message{
		Header: &pb.Header{
			Type:    t,
			From:    config.AppConfig.Address,
			Command: "ValidatorStarted",
		},
	}

	err := conn.SendMessage(message)
	if err != nil {
		fmt.Printf("Error when send started %v", err)
	}
}

func (conn *Connection) SendLeaderTick(tick *pb.POHTick) {
	protoRs, _ := proto.Marshal(tick)
	message := &pb.Message{
		Header: &pb.Header{
			Type:    "request",
			From:    config.AppConfig.Address,
			Command: "LeaderTick",
		},
		Body: protoRs,
	}

	err := conn.SendMessage(message)
	if err != nil {
		fmt.Printf("Error when send started %v", err)
	}
}

func (conn *Connection) SendVoteLeaderBlock(vote *pb.POHVote) {
	protoRs, _ := proto.Marshal(vote)
	message := &pb.Message{
		Header: &pb.Header{
			Type:    "request",
			From:    config.AppConfig.Address,
			Command: "VoteLeaderBlock",
		},
		Body: protoRs,
	}

	err := conn.SendMessage(message)
	if err != nil {
		fmt.Printf("Error when send started %v", err)
	}
}

func (conn *Connection) SendVoteResult(voteResult *pb.POHVoteResult) {
	protoRs, _ := proto.Marshal(voteResult)
	message := &pb.Message{
		Header: &pb.Header{
			Type:    "request",
			From:    config.AppConfig.Address,
			Command: "VoteResult",
		},
		Body: protoRs,
	}

	err := conn.SendMessage(message)
	if err != nil {
		fmt.Printf("Error when send started %v", err)
	}
}

func (conn *Connection) SendCheckedBlock(block *pb.CheckedBlock) {
	protoRs, _ := proto.Marshal(block)
	message := &pb.Message{
		Header: &pb.Header{
			Type:    "request",
			From:    config.AppConfig.Address,
			Command: "SendCheckedBlock",
		},
		Body: protoRs,
	}

	err := conn.SendMessage(message)
	if err != nil {
		fmt.Printf("Error when send started %v", err)
	}
}

func (conn *Connection) SendConfirmResult(confirmResult *pb.POHConfirmResult) {
	protoRs, _ := proto.Marshal(confirmResult)
	message := &pb.Message{
		Header: &pb.Header{
			Type:    "request",
			From:    config.AppConfig.Address,
			Command: "ConfirmResult",
		},
		Body: protoRs,
	}

	err := conn.SendMessage(message)
	if err != nil {
		fmt.Printf("Error when send started %v", err)
	}
}
