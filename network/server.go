package network

import (
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
)

type Server struct {
	Address string
	IP      string
	Port    int

	MessageHandler *MessageHandler
}

func (server *Server) Run(validatorConnections []*Connection) {
	log.Info(fmt.Sprintf("Starting server at port %d", server.Port))
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", server.IP, server.Port))
	if err != nil {
		log.Error(err)
	}
	defer listener.Close()

	go server.ConnectToOtherValidator(validatorConnections)
	go server.MessageHandler.UpdateLeaderIdx()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Error(err)
		}

		myConn := Connection{
			TCPConnection: conn,
		}
		server.MessageHandler.OnConnect(&myConn)
		go server.MessageHandler.HandleConnection(&myConn)
	}
}

func (server *Server) ConnectToOtherValidator(validatorConnections []*Connection) {
	for {
		if len(server.MessageHandler.Connections.NodeConnections) > 0 {
			for i, v := range validatorConnections {
				if _, ok := server.MessageHandler.Connections.ValidatorConnections[v.Address]; ok {
					// already connected
					log.Info("Already inited")
					continue
				}

				conn, err := net.Dial("tcp", fmt.Sprintf("%v:%v", v.IP, v.Port))
				if err != nil {
					log.Warn("Error when connect to %v:%v, wallet adress : %v", err)
				} else {
					validatorConnections[i].TCPConnection = conn
					server.MessageHandler.OnConnect(validatorConnections[i])
					go server.MessageHandler.HandleConnection(validatorConnections[i])
				}
			}
			break
		}
	}
}
