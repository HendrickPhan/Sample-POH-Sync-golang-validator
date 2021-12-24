package poh

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"example_poh.com/config"
	"example_poh.com/dataType"
	pb "example_poh.com/proto"
	log "github.com/sirupsen/logrus"
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

	go service.ReceiveTickFromNextLeaderTickChan()

	for {
		lastBlock := <-service.BlockChan
		// send leader index to this chan so message handler know what to do with incomming transactions
		service.LeaderIndexChan <- service.getCurrentLeaderIdx(lastBlock)

		// add to recorder to update poh block history
		fmt.Printf("Last block count: %v, type: %v\n", lastBlock.Count, lastBlock.Type)
		service.Recorder.AddBlock(lastBlock)
		if len(service.Recorder.Branches) > 0 {
			fmt.Printf("Branch total transaction: %v\n", service.Recorder.Branches[service.Recorder.MainBranchIdx].TotalTransaction)
		}
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
	tickTime := int(time.Second) / service.TickPerSecond
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
	tickTime := int(time.Second) / service.TickPerSecond
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
	for _, v := range service.Server.MessageHandler.ValidatorConnections {
		if v.Address != config.AppConfig.Address {
			v.SendLeaderTick(tick)
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
		go service.broadCastLeaderTick(tick)
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

func (service *POHService) validateLeaderTick(lastTick *pb.POHTick, tick *pb.POHTick) error {
	if lastTick.Count+1 != tick.Count {
		// panic(fmt.Sprintf("invalid tick count %v, %v", lastTick.Count+1, tick.Count))

		return fmt.Errorf("invalid tick count %v, %v", lastTick.Count+1, tick.Count)
	}
	if lastTick.Hashes[len(lastTick.Hashes)-1].Hash != tick.Hashes[0].LastHash {
		return fmt.Errorf("invalid tick hash %v, %v", lastTick.Hashes[len(lastTick.Hashes)-1].Hash, tick.Hashes[0].LastHash)
	}
	// validate POH in tick
	validatePOHChan := make(chan bool)
	exitChan := make(chan bool)

	for i := 0; i < config.AppConfig.NumberOfValidatePohRoutine; i++ {
		hashPerTick := service.HashPerSecond / service.TickPerSecond
		hashNeedValidatePerRoutine := int(math.Ceil(float64(hashPerTick) / float64(config.AppConfig.NumberOfValidatePohRoutine))) // ceil to not miss any transaction
		go func(i int) {
			validateFromIdx := i*hashNeedValidatePerRoutine - 1
			if validateFromIdx < 0 {
				validateFromIdx = 0
			}
			validateToIdx := (i + 1) * hashNeedValidatePerRoutine
			hashNeedValidate := tick.Hashes[validateFromIdx:validateToIdx]
			for i := 1; i < len(hashNeedValidate); i++ {
				select {
				case <-exitChan:
					return
				default:
					rightHash := service.generatePOHHash(hashNeedValidate[i].Transactions, hashNeedValidate[i-1])
					if rightHash.Hash != hashNeedValidate[i].Hash {
						validatePOHChan <- false
						return
					}
				}
			}
			validatePOHChan <- true
		}(i)
	}
	totalValidRoutine := 0
	for {
		valid := <-validatePOHChan
		if !valid {
			<-exitChan
			return errors.New("invalid poh")
		} else {
			totalValidRoutine++
		}
		if totalValidRoutine == config.AppConfig.NumberOfValidatePohRoutine {
			return nil
		}

	}
	// TODO: send to child to validate
}

func (service *POHService) isFutureTick(tick *pb.POHTick, ticks []*pb.POHTick, lastBlock *pb.POHBlock) bool {
	isFutureTick := false
	if len(ticks) > 0 {
		if tick.Count > ticks[len(ticks)-1].Count+1 {
			isFutureTick = true
		}
	} else {
		if tick.Count > lastBlock.Ticks[len(lastBlock.Ticks)-1].Count+1 {
			isFutureTick = true
		}
	}
	return isFutureTick
}

func (service *POHService) addFutureLeaderTick(futureTicks *[]*pb.POHTick, tick *pb.POHTick) {
	fmt.Printf("addFutureLeaderTick\n")
	*futureTicks = append(*futureTicks, tick)
	sort.Slice(*futureTicks, func(i, j int) bool {
		return (*futureTicks)[i].Count < (*futureTicks)[j].Count
	})
}

func (service *POHService) checkFutureLeaderTicks(futureTicks *[]*pb.POHTick, ticks *[]*pb.POHTick) {
	for {
		if len(*futureTicks) == 0 {
			return
		}
		tick := (*ticks)[len((*ticks))-1]
		if (*futureTicks)[0].Count == tick.Count+1 {
			err := service.validateLeaderTick(tick, (*futureTicks)[0])
			if err != nil {
				fmt.Printf("Error when validate leader tick: %v\n", err)
			} else {
				tick := (*futureTicks)[0]
				*ticks = append(*ticks, tick)
			}
			*futureTicks = (*futureTicks)[1:]
		} else {
			return
		}
	}
}

func (service *POHService) ReceiveTickFromNextLeaderTickChan() {
	for {
		tick := <-service.ReceiveNextLeaderTickChan
		service.mu.Lock()
		service.NextLeaderTicks = append(service.NextLeaderTicks, tick)
		service.mu.Unlock()

	}

}

func (service *POHService) HandleTicksInNextLeaderTicks(
	ticks *[]*pb.POHTick,
	lastBlock *pb.POHBlock,
	futureTicks *[]*pb.POHTick,
	totalTickGenerated *int,
) {
	for _, tick := range service.NextLeaderTicks {
		service.addTickToListLeaderTick(
			tick,
			ticks,
			lastBlock,
			futureTicks,
			totalTickGenerated,
		)
	}
	service.mu.Lock()
	service.NextLeaderTicks = []*pb.POHTick{}
	service.mu.Unlock()
}

func (service *POHService) addTickToListLeaderTick(
	tick *pb.POHTick,
	ticks *[]*pb.POHTick,
	lastBlock *pb.POHBlock,
	futureTicks *[]*pb.POHTick,
	totalTickGenerated *int,
) {
	*totalTickGenerated++
	if service.isFutureTick(tick, *ticks, lastBlock) {
		service.addFutureLeaderTick(futureTicks, tick)
		fmt.Printf("futureTicks %v \n", (*futureTicks)[0].Count)
		return
	}
	var err error
	if len(*ticks) > 0 { // if first tick then last tick is last tick of last block
		err = service.validateLeaderTick((*ticks)[len(*ticks)-1], tick)
	} else {
		err = service.validateLeaderTick(lastBlock.Ticks[len(lastBlock.Ticks)-1], tick)
	}
	if err != nil {
		fmt.Printf("Error when validate leader tick: %v\n", err)
		return
	}
	// TODO: handle casse that tick not come in order e.x: tick 2 come, then tick 1 come
	*ticks = append(*ticks, tick)
	service.checkFutureLeaderTicks(futureTicks, ticks)
}

func (service *POHService) HandleLeaderTick(
	lastBlock *pb.POHBlock,
	leaderBlockChan chan *pb.POHBlock,
	exitChan chan bool) {
	// This function use to handle tick data from leader
	totalTickGenerated := 0
	var ticks []*pb.POHTick
	var futureTicks []*pb.POHTick
	var tick *pb.POHTick

	// handle leader ticks
	for totalTickGenerated < service.TickPerSlot-1 {
		// handle tick received from next leader before confirmed block
		service.HandleTicksInNextLeaderTicks(
			&ticks,
			lastBlock,
			&futureTicks,
			&totalTickGenerated,
		)

		// append tick to create vote block
		tick = <-service.ReceiveLeaderTickChan
		// fmt.Printf("receive tick count %v\n", tick.Count)
		service.addTickToListLeaderTick(
			tick,
			&ticks,
			lastBlock,
			&futureTicks,
			&totalTickGenerated,
		)
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
	go service.HandleVoteResult(voteBlock, leaderBlockChan, exitChan)
}

func (service *POHService) SendVoteToLeader(lastBLock *pb.POHBlock, vote *pb.POHVote) {
	leader := service.getCurrentLeader(lastBLock)
	if leaderConn, ok := service.Server.MessageHandler.ValidatorConnections[leader.Address]; ok {
		leaderConn.SendVoteLeaderBlock(vote)
	} else {
		log.Error("Not found leader connection to send vote")
	}
}

func (service *POHService) HandleVoteResult(
	voteBlock *pb.POHBlock,
	leaderBlockChan chan *pb.POHBlock,
	exitChan chan bool,
) {
	// This function use to handle voted block data from leader
	select {
	case voteResult := <-service.ReceiveVoteResultChan:
		// TODO: validate sign in vote result
		if voteResult.Hash == voteBlock.Hash.Hash {
			leaderBlockChan <- voteBlock
		}
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
				service.BroadCastVoteResultToValidators(leaderBlock)
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

func (service *POHService) BroadCastVoteResultToValidators(votedBlock *pb.POHBlock) {
	voteResult := &pb.POHVoteResult{
		Hash:  votedBlock.Hash.Hash,
		Votes: votedBlock.Votes,
	}
	for _, v := range service.Server.MessageHandler.ValidatorConnections {
		if v.Address != config.AppConfig.Address {
			v.SendVoteResult(voteResult)
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
