package main

import (
	"example_poh.com/client"
	"example_poh.com/config"
	"example_poh.com/dataType"
	"example_poh.com/poh"
	"example_poh.com/server"
)

func runPoh(startPOHChan chan bool, leaderTickChan chan poh.POHTick, receiveVotedBlockChan chan poh.POHBlock, receiveValidatorVotesChan chan poh.POHVote) {
	start := <-startPOHChan
	if start {
		lastHash := poh.POHHash{
			Count:    1,
			LastHash: "",
			Hash:     "INIT_HASH",
		}
		lastTick := poh.POHTick{
			Hashes: []poh.POHHash{lastHash},
			Count:  1,
		}

		checkpoint := poh.POHBlock{
			Ticks: []poh.POHTick{lastTick},
			Count: 1,
			Type:  "leader",
			Hash:  lastHash,
		}

		pohRecorder := poh.POHRecorder{
			StartBlockCount: 1,
		}

		pohService := poh.POHService{
			Recorder:      pohRecorder,
			Checkpoint:    checkpoint,
			HashPerSecond: config.AppConfig.HashPerSecond,
			TickPerSecond: config.AppConfig.TickPerSecond,
			TickPerSlot:   config.AppConfig.TickPerSlot,
			TimeOutTicks:  config.AppConfig.TimeOutTicks,
		}
		validators := config.AppConfig.Validators
		// add self to validator list cuz this node is validator too
		validators = append(validators, dataType.Validator{
			Address: config.AppConfig.Address,
			Ip:      config.AppConfig.Ip,
			Port:    config.AppConfig.Port,
		})
		// call this function to sort validator in righ oder
		pohService.AddValidators(validators)
		go pohService.Run(leaderTickChan, receiveVotedBlockChan, receiveValidatorVotesChan)
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

func runServer(
	startPOHChan chan bool,
	validatorConnections map[string]*client.Client,
	receiveLeaderTickChan chan poh.POHTick,
	receiveVotedBlockChan chan poh.POHBlock,
	receiveValidatorVotesChan chan poh.POHVote,
) {
	handler := server.MessageHandler{
		StartPOHChan:              startPOHChan,
		ValidatorConnections:      initValidatorConnections(),
		OnlineValidator:           make(map[string]*client.Client),
		ReceiveLeaderTickChan:     receiveLeaderTickChan,
		ReceiveVotedBlockChan:     receiveVotedBlockChan,
		ReceiveValidatorVotesChan: receiveValidatorVotesChan,
	}
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
	receiveLeaderTickChan := make(chan poh.POHTick)
	receiveVotedBlockChan := make(chan poh.POHBlock)
	receiveValidatorVotesChan := make(chan poh.POHVote)

	startPOHChan := make(chan bool)
	validatorConnections := initValidatorConnections()

	go runPoh(startPOHChan, receiveLeaderTickChan, receiveVotedBlockChan, receiveValidatorVotesChan)
	go runServer(startPOHChan, validatorConnections, receiveLeaderTickChan, receiveVotedBlockChan, receiveValidatorVotesChan)
	<-finish
}
