package main

import (
	"example_poh.com/config"
	"example_poh.com/dataType"
	"example_poh.com/network"
	"example_poh.com/poh"
	pb "example_poh.com/proto"
)

func runPoh(
	startPOHChan chan bool,
	receiveleaderTickChan chan *pb.POHTick,
	receiveVoteChan chan *pb.POHVote,
	receiveVoteResult chan *pb.POHVoteResult,
	leaderIndexChan chan int,
	receiveCheckedBlockChan chan *pb.CheckedBlock,
	receiveNextLeaderTickChan chan *pb.POHTick,
	checkpoint *pb.POHBlock,
	server *network.Server,
) {
	start := <-startPOHChan
	if start {
		pohRecorder := poh.POHRecorder{
			StartBlockCount: 1,
		}

		pohService := poh.POHService{
			Server:                    server,
			Recorder:                  pohRecorder,
			Checkpoint:                checkpoint,
			HashPerSecond:             config.AppConfig.HashPerSecond,
			TickPerSecond:             config.AppConfig.TickPerSecond,
			TickPerSlot:               config.AppConfig.TickPerSlot,
			TimeOutTicks:              config.AppConfig.TimeOutTicks,
			BlockChan:                 make(chan *pb.POHBlock),
			ReceiveLeaderTickChan:     receiveleaderTickChan,
			ReceiveVoteChan:           receiveVoteChan,
			ReceiveVoteResultChan:     receiveVoteResult,
			LeaderIndexChan:           leaderIndexChan,
			ReceiveCheckedBlockChan:   receiveCheckedBlockChan,
			ReceiveNextLeaderTickChan: receiveNextLeaderTickChan,
		}
		// call this function to sort validator in righ oder
		pohService.AddValidators(initValidatorList())
		go pohService.Run()
	}
}

func initValidatorConnections() []network.Connection {
	validators := config.AppConfig.Validators
	var validatorConnections []network.Connection
	for _, validator := range validators {
		validatorConnections = append(validatorConnections, network.Connection{
			IP:      validator.Ip,
			Port:    validator.Port,
			Address: validator.Address,
		})
	}
	return validatorConnections
}

func initValidatorList() []dataType.Validator {
	validators := append([]dataType.Validator{}, config.AppConfig.Validators...) // copy slice so not doing any change to origin data
	validators = append(validators, dataType.Validator{
		Address: config.AppConfig.Address,
		Ip:      config.AppConfig.Ip,
		Port:    config.AppConfig.Port,
	})
	return validators
}

func runServer(
	startPOHChan chan bool,
	validatorConnections []network.Connection,
	receiveLeaderTickChan chan *pb.POHTick,
	receiveVoteChan chan *pb.POHVote,
	receiveVoteResult chan *pb.POHVoteResult,
	leaderIndexChan chan int,
	receiveCheckedBlockChan chan *pb.CheckedBlock,
	receiveNextLeaderTickChan chan *pb.POHTick,
	checkpoint *pb.POHBlock,
) *network.Server {

	initedConnectionsChan := make(chan network.Connection)
	removeConnectionChan := make(chan network.Connection)

	handler := network.MessageHandler{
		InitedConnectionsChan:     initedConnectionsChan,
		RemoveConnectionChan:      removeConnectionChan,
		StartPOHChan:              startPOHChan,
		ReceiveLeaderTickChan:     receiveLeaderTickChan,
		ReceiveVoteChan:           receiveVoteChan,
		ReceiveVoteResultChan:     receiveVoteResult,
		LeaderIndexChan:           leaderIndexChan,
		ReceiveCheckedBlockChan:   receiveCheckedBlockChan,
		ReceiveNextLeaderTickChan: receiveNextLeaderTickChan,
		ValidatorConnections:      make(map[string]*network.Connection),
	}
	handler.AddValidators(initValidatorList())
	handler.LeaderIndex = int((checkpoint.Count + 1) % int64(len(handler.Validators))) // last block count is 1
	server := network.Server{
		Address:               config.AppConfig.Address,
		IP:                    config.AppConfig.Ip,
		Port:                  config.AppConfig.Port,
		MessageHandler:        handler,
		InitedConnections:     make(map[string]network.Connection),
		InitedConnectionsChan: initedConnectionsChan,
		RemoveConnectionChan:  removeConnectionChan,
	}
	go server.Run(validatorConnections)
	return &server
}

func main() {
	finish := make(chan bool)
	receiveLeaderTickChan := make(chan *pb.POHTick)
	receiveVoteChan := make(chan *pb.POHVote)
	receiveVoteResult := make(chan *pb.POHVoteResult)
	leaderIndexChan := make(chan int)
	receiveCheckedBlockChan := make(chan *pb.CheckedBlock)
	receiveNextLeaderTickChan := make(chan *pb.POHTick)

	startPOHChan := make(chan bool)
	validatorConnections := initValidatorConnections()

	lastHash := &pb.POHHash{
		Count:    1,
		LastHash: "",
		Hash:     "INIT_HASH",
	}
	lastTick := &pb.POHTick{
		Hashes: []*pb.POHHash{lastHash},
		Count:  1,
	}

	checkpoint := &pb.POHBlock{
		Ticks: []*pb.POHTick{lastTick},
		Count: 1,
		Type:  "leader",
		Hash:  lastHash,
	}

	server := runServer(startPOHChan, validatorConnections, receiveLeaderTickChan, receiveVoteChan, receiveVoteResult, leaderIndexChan, receiveCheckedBlockChan, receiveNextLeaderTickChan, checkpoint)
	go runPoh(startPOHChan, receiveLeaderTickChan, receiveVoteChan, receiveVoteResult, leaderIndexChan, receiveCheckedBlockChan, receiveNextLeaderTickChan, checkpoint, server)
	<-finish
}
