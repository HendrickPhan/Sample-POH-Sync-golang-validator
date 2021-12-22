package client

import (
	"fmt"
	"math"
	"net"

	"example_poh.com/config"
	"github.com/google/uuid"

	pb "example_poh.com/proto"
	"google.golang.org/protobuf/proto"
)

type Client struct {
	Address  string
	IP       string
	Port     int
	NodeType string
}

func (client *Client) GetConnectionAddress() string {
	return fmt.Sprintf("%v:%v", client.IP, client.Port)
}

func (client *Client) GetWalletAddress() string {
	return client.Address
}

func (client *Client) SendMessage(message *pb.Message) error {
	body := append([]byte{}, message.Body...)
	maxBodySize := 50000
	totalPackage := math.Ceil(float64(len(body)) / float64(maxBodySize))
	if totalPackage == 0 {
		totalPackage = 1
	}
	message.Header.TotalPackage = int32(totalPackage)
	message.Header.Id = uuid.New().String()
	conn, err := net.Dial("udp", client.GetConnectionAddress())
	if err != nil {
		fmt.Printf("Error when dial UDP server %v", err)
		return err
	}
	for i := 0; int32(i) < message.Header.TotalPackage; i++ {
		var sendBody []byte
		if len(body) < maxBodySize {
			sendBody = body
		} else {
			sendBody = body[:maxBodySize]
			body = body[maxBodySize:]
		}
		sendMessage := &pb.Message{
			Header: message.Header,
			Body:   sendBody,
		}

		// b, err := json.Marshal(sendMessage)
		b, err := proto.Marshal(sendMessage)
		if err != nil {
			fmt.Printf("Error when marshal %v", err)
			return err
		}
		conn.Write(b)
	}
	conn.Close()

	return nil
}

func (client *Client) SendStarted(t string) {
	message := &pb.Message{
		Header: &pb.Header{
			Type:    t,
			From:    config.AppConfig.Address,
			Command: "ValidatorStarted",
		},
	}

	err := client.SendMessage(message)
	if err != nil {
		fmt.Printf("Error when send started %v", err)
	}
}

func (client *Client) SendLeaderTick(tick *pb.POHTick) {
	protoRs, _ := proto.Marshal(tick)
	message := &pb.Message{
		Header: &pb.Header{
			Type:    "request",
			From:    config.AppConfig.Address,
			Command: "LeaderTick",
		},
		Body: protoRs,
	}

	err := client.SendMessage(message)
	if err != nil {
		fmt.Printf("Error when send started %v", err)
	}
}

func (client *Client) SendVoteLeaderBlock(vote *pb.POHVote) {
	protoRs, _ := proto.Marshal(vote)
	message := &pb.Message{
		Header: &pb.Header{
			Type:    "request",
			From:    config.AppConfig.Address,
			Command: "VoteLeaderBlock",
		},
		Body: protoRs,
	}

	err := client.SendMessage(message)
	if err != nil {
		fmt.Printf("Error when send started %v", err)
	}
}

func (client *Client) SendVotedBlock(block *pb.POHBlock) {
	protoRs, _ := proto.Marshal(block)
	message := &pb.Message{
		Header: &pb.Header{
			Type:    "request",
			From:    config.AppConfig.Address,
			Command: "VotedBlock",
		},
		Body: protoRs,
	}

	err := client.SendMessage(message)
	if err != nil {
		fmt.Printf("Error when send started %v", err)
	}
}

func (client *Client) SendCheckedBlock(block *pb.CheckedBlock) {
	protoRs, _ := proto.Marshal(block)
	message := &pb.Message{
		Header: &pb.Header{
			Type:    "request",
			From:    config.AppConfig.Address,
			Command: "SendCheckedBlock",
		},
		Body: protoRs,
	}

	err := client.SendMessage(message)
	if err != nil {
		fmt.Printf("Error when send started %v", err)
	}
}
