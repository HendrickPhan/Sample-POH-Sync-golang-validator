package server

import (
	"encoding/json"
	"fmt"
	"net"

	"example_poh.com/dataType"
)

type Server struct {
	Address string
	IP      string
	Port    int
}

func (server *Server) GetConnectionAddress() string {
	return fmt.Sprintf("%v:%v", server.IP, server.Port)
}

func (server *Server) Run(handler MessageHandler) {
	p := make([]byte, 65535)
	addr := net.UDPAddr{
		Port: server.Port,
		IP:   net.ParseIP(server.IP),
	}
	ser, err := net.ListenUDP("udp", &addr)
	fmt.Printf("UDP server running \n")
	if err != nil {
		fmt.Printf("Some error %v\n", err)
		return
	}
	go func() {
		// TODO: find better way to manage this, like time out for pending, check hash, encrypt, v.v
		// but in dev process i just keep it simple
		pendingMessages := make(map[string]dataType.Message) // map between id and message

		for {
			n, _, err := ser.ReadFromUDP(p)
			// n, remoteaddr, err := ser.ReadFromUDP(p)
			// fmt.Printf("Read a message from %v %v \n", remoteaddr, n)
			if err != nil {
				fmt.Printf("Some error  %v", err)
				continue
			}
			var message dataType.Message
			json.Unmarshal(p[:n], &message)
			if pendingMessage, ok := pendingMessages[message.Header.Id]; ok {
				//do something here
				pendingMessage.Body += message.Body
				pendingMessage.Header.TotalReceived++
				pendingMessages[message.Header.Id] = pendingMessage
				if pendingMessage.Header.TotalPackage == pendingMessage.Header.TotalReceived {
					// create process messsage and remove from pending
					go handler.ProcessMessage(pendingMessage)
					delete(pendingMessages, message.Header.Id)
				}
			} else {
				if message.Header.TotalPackage == 1 {
					go handler.ProcessMessage(message)
				} else {
					message.Header.TotalReceived = 1
					pendingMessages[message.Header.Id] = message
				}
			}

		}
	}()
}
