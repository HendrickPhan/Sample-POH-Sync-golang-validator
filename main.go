package main

import (
	"example_poh.com/config"
	"example_poh.com/controllers"
	"example_poh.com/dataType"
	"example_poh.com/network"
	"example_poh.com/poh"
	pb "example_poh.com/proto"

	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
)

func runPoh(
	connections *network.Connections,
	receiveleaderTickChan chan *pb.POHTick,
	receiveVoteChan chan *pb.POHVote,
	receiveVoteResult chan *pb.POHVoteResult,
	leaderIndexChan chan int,
	receiveCheckedBlockChan chan *pb.CheckedBlock,
	receiveValidateTickResultChan chan *pb.POHValidateTickResult,
	checkpoint *pb.POHBlock,
	accountDB *leveldb.DB,
) {
	pohRecorder := poh.POHRecorder{
		StartBlockCount: 1,
	}

	pohService := poh.POHService{
		Connections:                   connections,
		AccountDB:                     accountDB,
		Recorder:                      pohRecorder,
		Checkpoint:                    checkpoint,
		HashPerSecond:                 config.AppConfig.HashPerSecond,
		TickPerSecond:                 config.AppConfig.TickPerSecond,
		TickPerSlot:                   config.AppConfig.TickPerSlot,
		TimeOutTicks:                  config.AppConfig.TimeOutTicks,
		CheckingLastHashes:            make(map[string]string),
		BlockChan:                     make(chan *pb.POHBlock),
		ReceiveLeaderTickChan:         receiveleaderTickChan,
		ReceiveVoteChan:               receiveVoteChan,
		ReceiveVoteResultChan:         receiveVoteResult,
		LeaderIndexChan:               leaderIndexChan,
		ReceiveCheckedBlockChan:       receiveCheckedBlockChan,
		ReceiveValidateTickResultChan: receiveValidateTickResultChan,
	}
	// call this function to sort validator in righ oder
	pohService.AddValidators(initValidatorList())
	go pohService.Run()
}

func initValidatorConnections() []*network.Connection {
	validators := config.AppConfig.Validators
	var validatorConnections []*network.Connection
	for _, validator := range validators {
		validatorConnections = append(validatorConnections, &network.Connection{
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

func initCheckPoinBlock() *pb.POHBlock {
	haveCheckPoint := false //TODO
	var checkPoinBlock *pb.POHBlock
	if haveCheckPoint {
		// load first block from check point
	} else {
		checkPoinBlock = controllers.GenerateGenesisBlock(config.AppConfig.GenesisBlockInfo)
	}
	return checkPoinBlock
}

func runServer(
	connections *network.Connections,
	validatorConnections []*network.Connection,
	receiveLeaderTickChan chan *pb.POHTick,
	receiveVoteChan chan *pb.POHVote,
	receiveVoteResult chan *pb.POHVoteResult,
	leaderIndexChan chan int,
	receiveCheckedBlockChan chan *pb.CheckedBlock,
	receiveValidateTickResultChan chan *pb.POHValidateTickResult,
	checkpoint *pb.POHBlock,
) {

	handler := network.MessageHandler{
		Connections:                   connections,
		ReceiveLeaderTickChan:         receiveLeaderTickChan,
		ReceiveVoteChan:               receiveVoteChan,
		ReceiveVoteResultChan:         receiveVoteResult,
		LeaderIndexChan:               leaderIndexChan,
		ReceiveCheckedBlockChan:       receiveCheckedBlockChan,
		ReceiveValidateTickResultChan: receiveValidateTickResultChan,
	}
	handler.AddValidators(initValidatorList())
	handler.LeaderIndex = int((checkpoint.Count + 1) % int64(len(handler.Validators))) // last block count is 1
	server := network.Server{
		Address:        config.AppConfig.Address,
		IP:             config.AppConfig.Ip,
		Port:           config.AppConfig.Port,
		MessageHandler: &handler,
	}
	go server.Run(validatorConnections)
}

func main() {
	finish := make(chan bool)
	receiveLeaderTickChan := make(chan *pb.POHTick)
	receiveVoteChan := make(chan *pb.POHVote)
	receiveVoteResult := make(chan *pb.POHVoteResult)
	leaderIndexChan := make(chan int)
	receiveCheckedBlockChan := make(chan *pb.CheckedBlock)
	receiveValidateTickResultChan := make(chan *pb.POHValidateTickResult)
	validatorConnections := initValidatorConnections()

	connections := &network.Connections{
		NodeConnections:      make(map[string]*network.Connection),
		ValidatorConnections: make(map[string]*network.Connection),
	}

	checkpointBlock := initCheckPoinBlock()

	// db
	accountDB, err := leveldb.OpenFile(config.AppConfig.AccountDBPath, nil)
	if err != nil {
		panic(err)
	}
	defer accountDB.Close()

	go runServer(
		connections,
		validatorConnections,
		receiveLeaderTickChan,
		receiveVoteChan,
		receiveVoteResult,
		leaderIndexChan,
		receiveCheckedBlockChan,
		receiveValidateTickResultChan,
		checkpointBlock,
	)

	for {
		if len(connections.ValidatorConnections) == len(config.AppConfig.Validators) && len(connections.NodeConnections) > 0 {
			log.Infof("RUN POH\n")
			go runPoh(
				connections,
				receiveLeaderTickChan,
				receiveVoteChan,
				receiveVoteResult,
				leaderIndexChan,
				receiveCheckedBlockChan,
				receiveValidateTickResultChan,
				checkpointBlock,
				accountDB,
			)
			break
		}
	}

	<-finish
}
