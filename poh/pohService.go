package poh

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"example_poh.com/client"
	"example_poh.com/config"
	"example_poh.com/dataType"
	pb "example_poh.com/proto"
)

func (service *POHService) Run() {
	go func() {
		service.BlockChan <- service.Checkpoint
	}()

	go func() { // receive transaction from server
		for {
			// I think maybe need to lock here if have error about sync block from server to recorder and service
			checkedBlock := <-service.ReceiveCheckedBlockChan
			service.Recorder.AddTransactionFromCheckedBlock(checkedBlock)
		}
	}()

	for {
		lastBlock := <-service.BlockChan
		// send leader index to this chan so message handler know what to do with incomming transactions
		service.LeaderIndexChan <- service.getCurrentLeaderIdx(lastBlock)

		// add to recorder to update poh block history
		fmt.Printf("Last block count: %v, type: %v\n", lastBlock.Count, lastBlock.Type)
		service.Recorder.AddBlock(lastBlock)
		// get main branch last block to start working on
		mainBranchLastBlock := service.Recorder.GetMainBranchLastBlock()

		// check from last block that this block will be leader
		if service.isLeader(mainBranchLastBlock) {
			go service.RunAsLeader(mainBranchLastBlock)
		} else {
			go service.RunAsValidator(mainBranchLastBlock)
		}
	}
}

func (service *POHService) RunAsLeader(lastBlock *pb.POHBlock) {
	fmt.Printf("Run as leader\n")
	exitChan := make(chan bool)
	defer func() { // close all channel before exit
		exitChan <- true
	}()

	virtualBlockChan := make(chan *pb.POHBlock)
	leaderBlockChan := make(chan *pb.POHBlock)
	go service.CreateVirtualBlock(lastBlock, virtualBlockChan)
	go service.CreateLeaderBlock(lastBlock, leaderBlockChan, exitChan)
	var virtualBlock *pb.POHBlock
	var leaderBlock *pb.POHBlock
	tickTime := 1000000000 / service.TickPerSecond
	timeOut := time.Now().UnixNano() + int64(tickTime*(service.TickPerSlot+service.TimeOutTicks))
	for {
		select {
		case virtual := <-virtualBlockChan:
			virtualBlock = virtual
		case leader := <-leaderBlockChan:
			leaderBlock = leader
			service.BlockChan <- leaderBlock
			return
		default:
		}

		if time.Now().UnixNano() > timeOut { // which mean leader not send block in time so this node vote for virtual
			fmt.Printf("Self is leader but Time out\n")
			service.BlockChan <- virtualBlock
			return
		}
	}
}

func (service *POHService) RunAsValidator(lastBlock *pb.POHBlock) {
	fmt.Printf("Run as validator\n")
	exitChan := make(chan bool)
	defer func() { // close all channel before exit
		exitChan <- true
	}()
	virtualBlockChan := make(chan *pb.POHBlock)
	leaderBlockChan := make(chan *pb.POHBlock)

	go service.CreateVirtualBlock(lastBlock, virtualBlockChan)
	go service.HandleLeaderTick(lastBlock, leaderBlockChan, exitChan)
	var virtualBlock *pb.POHBlock
	tickTime := 1000000000 / service.TickPerSecond
	timeOut := time.Now().UnixNano() + int64(tickTime*(service.TickPerSlot+service.TimeOutTicks))
	for {
		select {
		case virtual := <-virtualBlockChan:
			virtualBlock = virtual
		case leaderBlock := <-leaderBlockChan:
			service.BlockChan <- leaderBlock
			return
		default:
		}

		if time.Now().UnixNano() > timeOut { // which mean leader not send block in time so this node vote for virtual
			fmt.Printf("Current leader has Timeout, %v\n", virtualBlock.Count)
			service.BlockChan <- virtualBlock
			return
		}
	}
}

func (service *POHService) CreateVirtualBlock(lastBlock *pb.POHBlock, virtualBlockChan chan *pb.POHBlock) {
	// This function use to create virtual block when is not a leader
	totalTickGenerated := 0
	lastTick := lastBlock.Ticks[len(lastBlock.Ticks)-1]
	var ticks []*pb.POHTick
	for totalTickGenerated < service.TickPerSlot {
		tick := service.CreateTick(lastTick, false)
		ticks = append(ticks, tick)
		lastTick = tick
		totalTickGenerated++
	}
	lastHash := lastTick.Hashes[len(lastTick.Hashes)-1]
	// Gen virtual block
	block := &pb.POHBlock{
		Ticks: ticks,
		Count: lastBlock.Count + 1,
		Type:  "virtual",
		Hash:  lastHash,
	}
	virtualBlockChan <- block
}

func (service *POHService) CreateLeaderTick(lastTick *pb.POHTick) *pb.POHTick {
	// This function use to create tick when is a leader
	tick := service.CreateTick(lastTick, true)
	return tick
}

func (service *POHService) broadCastLeaderTick(tick *pb.POHTick) {
	for _, v := range service.Validators {
		if v.Address != config.AppConfig.Address {
			client := client.Client{
				Address: v.Address,
				IP:      v.Ip,
				Port:    v.Port,
			}

			client.SendLeaderTick(tick)
		}
	}
}

func (service *POHService) CreateTick(lastTick *pb.POHTick, takeTransaction bool) *pb.POHTick {

	hashPerTick := service.HashPerSecond / service.TickPerSecond
	totalHashGenerated := 0
	lastHash := lastTick.Hashes[len(lastTick.Hashes)-1]
	var hashes []*pb.POHHash
	for totalHashGenerated < hashPerTick {
		var transactions []*pb.Transaction
		if takeTransaction {
			// TODO: take transaction from recorder
			transactions = service.Recorder.TakeTransactions(config.AppConfig.TransactionPerHash)
		}
		hash := service.generatePOHHash(transactions, lastHash)
		hashes = append(hashes, hash)
		lastHash = hash
		totalHashGenerated++
	}
	tick := &pb.POHTick{
		Hashes: hashes,
		Count:  lastTick.Count + 1,
	}
	return tick
}

func (service *POHService) createLeaderBlockWithSelfGenLastTick(lastBlock *pb.POHBlock, ticks []*pb.POHTick) *pb.POHBlock {
	previousLastTick := ticks[len(ticks)-1]
	lastTick := service.CreateTick(previousLastTick, false)
	ticks = append(ticks, lastTick)
	lastHash := lastTick.Hashes[len(lastTick.Hashes)-1]
	leaderBlock := &pb.POHBlock{
		Ticks: ticks,
		Count: lastBlock.Count + 1,
		Type:  "leader",
		Hash:  lastHash,
		Votes: []*pb.POHVote{
			{
				Address: config.AppConfig.Address,
				Hash:    lastHash.Hash,
			},
		},
	}
	return leaderBlock
}

func (service *POHService) CreateLeaderBlock(
	lastBlock *pb.POHBlock,
	leaderBlockChan chan *pb.POHBlock,
	exitChan chan bool,
) {
	// This function use to create block when is a leader
	totalTickGenerated := 0
	lastTick := lastBlock.Ticks[len(lastBlock.Ticks)-1]
	var ticks []*pb.POHTick

	// throttle tick
	var tickEnd int64

	for totalTickGenerated < service.TickPerSlot {
		if time.Now().UnixNano()-tickEnd < int64(1000000000/service.TickPerSecond) { //1000000000 is 1 second in nano second
			time.Sleep(time.Duration((int64(1000000000/service.TickPerSecond) - (time.Now().UnixNano() - tickEnd) - 100))) // 100 is just a addition to make it not to tight
			continue
		}

		tick := service.CreateLeaderTick(lastTick)
		service.broadCastLeaderTick(tick)
		ticks = append(ticks, tick)
		lastTick = tick
		totalTickGenerated++
		// TODO: broad cast tick to validator
		if totalTickGenerated == service.TickPerSlot-1 { // last tick so we wait for votes from validators to create block
			// gen leader block and collect vote from validator to
			// gen last tick () with out taking any transaction because orther validator will do the same to create block
			leaderBlock := service.createLeaderBlockWithSelfGenLastTick(lastBlock, ticks)
			go service.HandleValidatorVotes(leaderBlock, leaderBlockChan, exitChan)
			return
		}

		tickEnd = time.Now().UnixNano()
	}
}

func (service *POHService) HandleLeaderTick(
	lastBlock *pb.POHBlock,
	leaderBlockChan chan *pb.POHBlock,
	exitChan chan bool) {
	// This function use to handle tick data from leader
	totalTickGenerated := 0
	var ticks []*pb.POHTick
	for totalTickGenerated < service.TickPerSlot-1 {
		// append tick to create vote block
		tick := <-service.ReceiveLeaderTickChan
		// TODO: validate tick data ex: does transaction valid, does tick come from leader, does tick last hash exist, v.v
		// TODO: handle casse that tick not come in order e.x: tick 2 come, then tick 1 come
		ticks = append(ticks, tick)
		totalTickGenerated++
	}

	// create vote and send to leader
	// generate last tick and vote block
	voteBlock := service.createLeaderBlockWithSelfGenLastTick(lastBlock, ticks)
	vote := &pb.POHVote{
		Hash:    voteBlock.Hash.Hash,
		Address: config.AppConfig.Address,
		Sign:    "TODO", // TODO: sign vote
	}
	// send vote to leader
	service.SendVoteToLeader(lastBlock, vote)
	go service.HandleVoteResult(voteBlock, service.ReceiveVotedBlockChan, leaderBlockChan, exitChan)
}

func (service *POHService) SendVoteToLeader(lastBLock *pb.POHBlock, vote *pb.POHVote) {
	leader := service.getCurrentLeader(lastBLock)
	client := client.Client{
		Address: leader.Address,
		IP:      leader.Ip,
		Port:    leader.Port,
	}

	client.SendVoteLeaderBlock(vote)
}

func (service *POHService) HandleVoteResult(
	voteBlock *pb.POHBlock,
	receiveVotedBlockChan chan *pb.POHBlock,
	leaderBlockChan chan *pb.POHBlock,
	exitChan chan bool,
) {
	// This function use to handle voted block data from leader
	select {
	case block := <-receiveVotedBlockChan:
		// TODO: Validate data in block ex: enough sign of orther validator
		leaderBlockChan <- block
	case <-exitChan:
		return
	}
}

func (service *POHService) HandleValidatorVotes(
	leaderBlock *pb.POHBlock,
	leaderBlockChan chan *pb.POHBlock,
	exitChan chan bool) {
	for {
		select {
		case vote := <-service.ReceiveVoteChan:
			if vote.Hash == leaderBlock.Hash.Hash {
				leaderBlock.Votes = append(leaderBlock.Votes, vote)
			}
			if len(leaderBlock.Votes) >= int((math.Ceil(float64(len(service.Validators)) * (2.0 / 3.0)))) {
				service.BroadCastVotedBlockToValidators(leaderBlock)
				leaderBlockChan <- leaderBlock
				return
			}
		case exit := <-exitChan:
			if exit {
				return
			}
		}
	}
}

func (service *POHService) BroadCastVotedBlockToValidators(votedBlock *pb.POHBlock) {
	for _, v := range service.Validators {
		if v.Address != config.AppConfig.Address {
			client := client.Client{
				Address: v.Address,
				IP:      v.Ip,
				Port:    v.Port,
			}

			client.SendVotedBlock(votedBlock)
		}
	}
}

func (service *POHService) getNodeIdx() int {
	nodeIdx := 0
	for i, v := range service.Validators {
		if v.Address == config.AppConfig.Address {
			nodeIdx = i
			break
		}
	}
	return nodeIdx
}

func (service *POHService) isLeader(lastBlock *pb.POHBlock) bool {
	return int((lastBlock.Count+1)%int64(len(service.Validators))) == service.getNodeIdx()
}

func (service *POHService) isNextLeader(lastBlock *pb.POHBlock) bool {
	return int((lastBlock.Count+1)%int64(len(service.Validators))) == service.getNodeIdx()-1
}

func (service *POHService) getCurrentLeaderIdx(lastBlock *pb.POHBlock) int {
	return int((lastBlock.Count + 1) % int64(len(service.Validators)))
}

func (service *POHService) getCurrentLeader(lastBlock *pb.POHBlock) dataType.Validator {
	leaderIdx := service.getCurrentLeaderIdx(lastBlock)
	return service.Validators[leaderIdx]
}

func (service *POHService) generatePOHHash(transactions []*pb.Transaction, lastHash *pb.POHHash) *pb.POHHash {
	h := sha256.New()
	transactionsHashString := ""
	for _, transaction := range transactions {
		transactionsHashString += "|" + transaction.Hash
	}

	h.Write([]byte(strconv.FormatInt(lastHash.Count, 10) + lastHash.Hash + transactionsHashString))

	result := &pb.POHHash{
		Count:        lastHash.Count + 1,
		LastHash:     lastHash.Hash,
		Hash:         hex.EncodeToString(h.Sum(nil)),
		Transactions: transactions,
	}
	service.HashStack = append(service.HashStack, result)
	return result
}

func (service *POHService) AddValidators(validators []dataType.Validator) {
	sort.Slice(validators, func(i, j int) bool {
		return validators[i].Address < validators[j].Address
	})

	service.Validators = validators
}
