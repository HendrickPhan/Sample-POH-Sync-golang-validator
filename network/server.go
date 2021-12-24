package network

import (
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
)

type Server struct {
	Address               string
	IP                    string
	Port                  int
	MessageHandler        MessageHandler
	UnInitedConnections   []Connection
	InitedConnections     map[string]Connection
	InitedConnectionsChan chan Connection
	RemoveConnectionChan  chan Connection
}

func (server *Server) Run(validatorConnections []Connection) {
	log.Info(fmt.Sprintf("Starting server at port %d", server.Port))
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", server.IP, server.Port))
	if err != nil {
		log.Error(err)
	}
	defer listener.Close()

	go server.handleInitedConnectionChan()
	go server.handleRemoveConnectionChan()
	go server.ConnectToServers(validatorConnections)
	go server.MessageHandler.UpdateLeaderIdx()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Error(err)
		}

		myConn := Connection{
			TCPConnection: conn,
		}
		server.MessageHandler.OnConnect(myConn)
		go server.MessageHandler.HandleConnection(myConn)
	}
}

func (server *Server) handleInitedConnectionChan() {
	for {
		con := <-server.InitedConnectionsChan
		server.InitedConnections[con.Address] = con
		log.Info(fmt.Sprintf("Inited Connection %v", len(server.InitedConnections)))
	}
}

func removeUnInitedConnection(s []Connection, i int) []Connection {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func (server *Server) handleRemoveConnectionChan() {
	for {
		con := <-server.InitedConnectionsChan
		delete(server.InitedConnections, con.Address)
		for i, v := range server.UnInitedConnections {
			if v == con {
				server.UnInitedConnections = removeUnInitedConnection(server.UnInitedConnections, i)
			}
		}
	}
}

func (server *Server) ConnectToServers(connections []Connection) {
	for _, v := range connections {
		if _, ok := server.InitedConnections[v.Address]; ok {
			// already connected
			log.Info("Already inited")
			continue
		}

		conn, err := net.Dial("tcp", fmt.Sprintf("%v:%v", v.IP, v.Port))
		if err != nil {
			log.Warn("Error when connect to %v:%v, wallet adress : %v", err)
		} else {
			v.TCPConnection = conn
			server.MessageHandler.OnConnect(v)
			go server.MessageHandler.HandleConnection(v)
		}
	}
}
