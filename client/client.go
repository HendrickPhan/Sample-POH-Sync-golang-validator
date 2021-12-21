package client

import (
	"encoding/json"
	"fmt"
	"math"
	"net"

	"example_poh.com/config"
	"example_poh.com/dataType"
	"github.com/google/uuid"
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

func (client *Client) SendMessage(message dataType.Message) error {
	maxBodySize := 50000
	bytesBody := []byte(message.Body)
	totalPackage := math.Ceil(float64(len(bytesBody)) / float64(maxBodySize))
	if totalPackage == 0 {
		totalPackage = 1
	}
	message.Header.TotalPackage = int(totalPackage)
	message.Header.Id = uuid.New().String()

	for i := 0; i < message.Header.TotalPackage; i++ {
		var sendBody []byte
		if len(bytesBody) < maxBodySize {
			sendBody = bytesBody
		} else {
			sendBody = bytesBody[:maxBodySize]
			bytesBody = bytesBody[maxBodySize:]
		}
		sendMessage := dataType.Message{
			Header: message.Header,
			Body:   string(sendBody),
		}

		b, err := json.Marshal(sendMessage)
		if err != nil {
			fmt.Printf("Error when unmarshal %v", err)
			return err
		}
		conn, err := net.Dial("udp", client.GetConnectionAddress())
		if err != nil {
			fmt.Printf("Error when dial UDP server %v", err)
			return err
		}
		conn.Write(b)
		conn.Close()
	}

	return nil
}

func (client *Client) SendStarted(t string) {
	message := dataType.Message{
		Header: dataType.Header{
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

func (client *Client) SendLeaderTick(tick interface{}) {
	jsonRs, _ := json.Marshal(tick)
	message := dataType.Message{
		Header: dataType.Header{
			Type:    "request",
			From:    config.AppConfig.Address,
			Command: "LeaderTick",
		},
		Body: string(jsonRs),
	}

	err := client.SendMessage(message)
	if err != nil {
		fmt.Printf("Error when send started %v", err)
	}
}

func (client *Client) SendVoteLeaderBlock(vote interface{}) {
	jsonRs, _ := json.Marshal(vote)
	message := dataType.Message{
		Header: dataType.Header{
			Type:    "request",
			From:    config.AppConfig.Address,
			Command: "VoteLeaderBlock",
		},
		Body: string(jsonRs),
	}

	err := client.SendMessage(message)
	if err != nil {
		fmt.Printf("Error when send started %v", err)
	}
}

func (client *Client) SendVotedBlock(block interface{}) {
	jsonRs, _ := json.Marshal(block)
	message := dataType.Message{
		Header: dataType.Header{
			Type:    "request",
			From:    config.AppConfig.Address,
			Command: "VotedBlock",
		},
		Body: string(jsonRs),
	}

	err := client.SendMessage(message)
	if err != nil {
		fmt.Printf("Error when send started %v", err)
	}
}
