package main

import (
	"example_poh.com/client"
	"example_poh.com/config"
	"example_poh.com/dataType"
	"example_poh.com/poh"
	pb "example_poh.com/proto"
	"example_poh.com/server"
)

func runPoh(
	startPOHChan chan bool,
	receiveleaderTickChan chan *pb.POHTick,
	receiveVotedBlockChan chan *pb.POHBlock,
	receiveVoteChan chan *pb.POHVote,
	leaderIndexChan chan int,
	receiveCheckedBlockChan chan *pb.CheckedBlock,
) {
	start := <-startPOHChan
	if start {
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

		pohRecorder := poh.POHRecorder{
			StartBlockCount: 1,
		}

		pohService := poh.POHService{
			Recorder:                pohRecorder,
			Checkpoint:              checkpoint,
			HashPerSecond:           config.AppConfig.HashPerSecond,
			TickPerSecond:           config.AppConfig.TickPerSecond,
			TickPerSlot:             config.AppConfig.TickPerSlot,
			TimeOutTicks:            config.AppConfig.TimeOutTicks,
			BlockChan:               make(chan *pb.POHBlock),
			ReceiveLeaderTickChan:   receiveleaderTickChan,
			ReceiveVotedBlockChan:   receiveVotedBlockChan,
			ReceiveVoteChan:         receiveVoteChan,
			LeaderIndexChan:         leaderIndexChan,
			ReceiveCheckedBlockChan: receiveCheckedBlockChan,
		}
		// call this function to sort validator in righ oder
		pohService.AddValidators(initValidatorList())
		go pohService.Run()
	}
}

func initValidatorConnections() map[string]*client.Client {
	validators := config.AppConfig.Validators
	validatorConnections := make(map[string]*client.Client)
	for _, validator := range validators {
		validatorConnections[validator.Address] = &client.Client{
			IP:       validator.Ip,
			Port:     validator.Port,
			Address:  validator.Address,
			NodeType: "validator",
		}
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
	validatorConnections map[string]*client.Client,
	receiveLeaderTickChan chan *pb.POHTick,
	receiveVotedBlockChan chan *pb.POHBlock,
	receiveVoteChan chan *pb.POHVote,
	leaderIndexChan chan int,
	receiveCheckedBlockChan chan *pb.CheckedBlock,
) {
	handler := server.MessageHandler{
		StartPOHChan:            startPOHChan,
		ValidatorConnections:    initValidatorConnections(),
		OnlineValidator:         make(map[string]*client.Client),
		ReceiveLeaderTickChan:   receiveLeaderTickChan,
		ReceiveVotedBlockChan:   receiveVotedBlockChan,
		ReceiveVoteChan:         receiveVoteChan,
		LeaderIndexChan:         leaderIndexChan,
		ReceiveCheckedBlockChan: receiveCheckedBlockChan,
	}
	handler.AddValidators(initValidatorList())

	server := server.Server{
		Address: config.AppConfig.Address,
		IP:      config.AppConfig.Ip,
		Port:    config.AppConfig.Port,
	}
	server.Run(handler)
	go sendStartedToAllValidator(validatorConnections)
}

func sendStartedToAllValidator(validatorConnections map[string]*client.Client) {
	for _, v := range validatorConnections {
		if v.GetWalletAddress() != config.AppConfig.Address { // send to other node, not yourself
			v.SendStarted("request")
		}
	}
}

func main() {
	finish := make(chan bool)
	receiveLeaderTickChan := make(chan *pb.POHTick)
	receiveVotedBlockChan := make(chan *pb.POHBlock)
	receiveVoteChan := make(chan *pb.POHVote)
	leaderIndexChan := make(chan int)
	receiveCheckedBlockChan := make(chan *pb.CheckedBlock)

	startPOHChan := make(chan bool)
	validatorConnections := initValidatorConnections()

	go runPoh(startPOHChan, receiveLeaderTickChan, receiveVotedBlockChan, receiveVoteChan, leaderIndexChan, receiveCheckedBlockChan)
	go runServer(startPOHChan, validatorConnections, receiveLeaderTickChan, receiveVotedBlockChan, receiveVoteChan, leaderIndexChan, receiveCheckedBlockChan)
	<-finish
}
