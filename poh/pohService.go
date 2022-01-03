package poh

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"example_poh.com/config"
	"example_poh.com/dataType"
	pb "example_poh.com/proto"
	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
	"google.golang.org/protobuf/proto"
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
		transactions := GetTransactions(lastBlock)
		newAccountDatas := service.GetNewAccountDatas(transactions)
		service.UpdateAccountDB(newAccountDatas)
		service.BroadCastConfirmResultToChildrens(transactions, newAccountDatas, lastBlock.Ticks[len(lastBlock.Ticks)-1])
		// clear checking last hashes cuz all has been updated
		service.CheckingLastHashes = make(map[string]string)

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
	if lastBlock.Count == 1 {
		// temporary fix for warmup leader when start POH
		timeOut += 1000 * int64(tickTime*(service.TickPerSlot+service.TimeOutTicks))
	}
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
	go service.HandleValidateTickResultFromChildrenNode(lastBlock, leaderBlockChan, exitChan)

	var virtualBlock *pb.POHBlock
	tickTime := int(time.Second) / service.TickPerSecond
	timeOut := time.Now().UnixNano() + int64(tickTime*(service.TickPerSlot+service.TimeOutTicks))
	if lastBlock.Count == 1 {
		// temporary fix for warmup leader when start POH
		timeOut += 1000 * int64(tickTime*(service.TickPerSlot+service.TimeOutTicks))
	}
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
	for _, v := range service.Connections.ValidatorConnections {
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
			transactions = service.Recorder.TakeTransactions(config.AppConfig.TransactionPerHash)
			transactions = service.validateTransactionLastHashes(transactions)
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

func (service *POHService) validateTransactionLastHashes(transactions []*pb.Transaction) []*pb.Transaction {
	var validatedTransactions []*pb.Transaction
	for _, transaction := range transactions {
		if _, ok := service.CheckingLastHashes[transaction.FromAddress]; !ok {
			bAccountData, err := service.AccountDB.Get([]byte(transaction.FromAddress), nil)
			if err != nil {
				log.Warn("Account have no data but create send transaction. Address: %v")
				continue
			}
			accountData := &pb.AccountData{}
			proto.Unmarshal(bAccountData, accountData)

			service.CheckingLastHashes[transaction.FromAddress] = accountData.LastHash
		}
		if transaction.PreviousData.Hash == service.CheckingLastHashes[transaction.FromAddress] {
			validatedTransactions = append(validatedTransactions, transaction)
		}
	}
	return validatedTransactions
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

func (service *POHService) SendVoteToLeader(lastBLock *pb.POHBlock, vote *pb.POHVote) {
	leader := service.getCurrentLeader(lastBLock)
	if leaderConn, ok := service.Connections.ValidatorConnections[leader.Address]; ok {
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
				log.Infof("Broad cast result %v\n", vote.Address)
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
	for _, v := range service.Connections.ValidatorConnections {
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
	return result
}

func (service *POHService) AddValidators(validators []dataType.Validator) {
	sort.Slice(validators, func(i, j int) bool {
		return validators[i].Address < validators[j].Address
	})

	service.Validators = validators
}

func (service *POHService) BroadCastTickToChildrenNodes(tick *pb.POHTick, merkelRoot string) {
	for _, nodeConnections := range service.Connections.NodeConnections {
		go nodeConnections.SendLeaderTick(tick)
	}
}

func (service *POHService) HandleValidateTickResultFromChildrenNode(lastBlock *pb.POHBlock, leaderBlockChan chan *pb.POHBlock, exitChan chan bool) {
	var validTicks []*pb.POHTick
	validTickSigns := make(map[string]POHValidTickSigns)
	// handle leader ticks
	for len(validTicks) < service.TickPerSlot-1 {
		select {
		case <-exitChan:
			return
		case validateTickResult := <-service.ReceiveValidateTickResultChan:
			if validateTickResult.Valid {
				tickHash := GetTickHash(validateTickResult.Tick)
				if _, ok := validTickSigns[tickHash]; ok {
					validTickSigns[tickHash].Signs[validateTickResult.From] = validateTickResult.Sign
				} else {
					validTickSigns[tickHash] = POHValidTickSigns{
						Tick: validateTickResult.Tick,
						Signs: map[string]string{
							validateTickResult.From: validateTickResult.Sign,
						},
					}
				}

				if len(validTickSigns[tickHash].Signs) >= int(math.Ceil(float64(2.0/3.0)*float64(len(service.Connections.NodeConnections)))) {
					validTicks = append(validTicks, validTickSigns[tickHash].Tick)
				}
			}
		}
	}

	sort.Slice(validTicks, func(i, j int) bool {
		return validTicks[i].Count < validTicks[j].Count
	})
	// TODO: check first tick is correct and all tick received have all tick count from start to end

	// create vote and send to leader
	// generate last tick and vote block
	voteBlock := service.createLeaderBlockWithSelfGenLastTick(lastBlock, validTicks)

	vote := &pb.POHVote{
		Hash:    voteBlock.Hash.Hash,
		Address: config.AppConfig.Address,
		Sign:    "TODO", // TODO: sign vote
	}
	// send vote to leader
	service.SendVoteToLeader(lastBlock, vote)
	go service.HandleVoteResult(voteBlock, leaderBlockChan, exitChan)
}

func (service *POHService) GetNewAccountDatas(transactions []*pb.Transaction) map[string]*pb.AccountData {
	newAccountData := make(map[string]*pb.AccountData)

	for _, transaction := range transactions {
		sendAmount := transaction.PreviousData.Balance + transaction.PendingUse - transaction.Balance
		// update pending balance of sender
		if _, ok := newAccountData[transaction.FromAddress]; !ok {
			bAccountData, _ := service.AccountDB.Get([]byte(transaction.FromAddress), nil)
			accountData := &pb.AccountData{}
			proto.Unmarshal(bAccountData, accountData)
			newAccountData[transaction.FromAddress] = accountData
		}
		newAccountData[transaction.FromAddress].PendingBalance -= transaction.PendingUse
		newAccountData[transaction.FromAddress].Balance = transaction.Balance
		newAccountData[transaction.FromAddress].LastHash = transaction.Hash

		// update pending balance of receiver
		if _, ok := newAccountData[transaction.ToAddress]; !ok {
			bAccountData, err := service.AccountDB.Get([]byte(transaction.ToAddress), nil)
			accountData := &pb.AccountData{}
			if err != nil {
				accountData.Address = transaction.ToAddress
			} else {
				proto.Unmarshal(bAccountData, accountData)
			}
			newAccountData[transaction.ToAddress] = accountData
		}
		newAccountData[transaction.ToAddress].PendingBalance += sendAmount
	}
	return newAccountData
}

func (service *POHService) UpdateAccountDB(newAccountDatas map[string]*pb.AccountData) {
	batch := new(leveldb.Batch)
	for _, v := range newAccountDatas {
		b, _ := proto.Marshal(v)
		batch.Put([]byte(v.Address), b)
	}
	err := service.AccountDB.Write(batch, nil)
	if err != nil {
		log.Errorf("Error when update account db %v\n", err)
	}
}

func (service *POHService) BroadCastConfirmResultToChildrens(transactions []*pb.Transaction, newAccountDatas map[string]*pb.AccountData, lastTick *pb.POHTick) {
	// extract last hash of account in block
	confirmResult := &pb.POHConfirmResult{
		LastTick:     lastTick,
		AccountDatas: newAccountDatas,
		Transactions: transactions,
	}

	for _, v := range service.Connections.NodeConnections {
		v.SendConfirmResult(confirmResult)
	}
}
