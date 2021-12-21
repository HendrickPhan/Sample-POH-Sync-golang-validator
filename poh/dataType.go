package poh

import "example_poh.com/dataType"

type POHHash struct {
	Count        int64                  `json:"count"`
	LastHash     string                 `json:"last_hash"`
	Hash         string                 `json:"hash"`
	Transactions []dataType.Transaction `json:"transactions"`
}

type POHTick struct {
	Hashes []POHHash `json:"hashes"`
	Count  int64     `json:"count"`
}

type POHBlock struct {
	Ticks []POHTick `json:"ticks"`
	Count int64     `json:"count"`
	Type  string    `json:"type"`
	Hash  POHHash   `json:"hash"`
	Votes []POHVote `json:"votes"`
}

type POHVote struct {
	Hash    string `json:"hash"`
	Address string `json:"address"`
	Sign    string `json:"sign"`
}

type POHBranch struct {
	Blocks           []POHBlock `json:"blocks"`
	TotalTransaction int        `json:"total_transaction"`
	TotalLeaderBlock int        `json:"total_leader_block"`
}

type POHRecorder struct {
	Transactions    dataType.Transaction // pending transactions
	MainBranchIdx   int                  // index of main branch
	StartBlockCount int64                // indicate from which block count this node will process, else remove
	Branches        []POHBranch          // save forks
}

type POHService struct {
	Checkpoint    POHBlock             // where new tick will start hash
	HashStack     []POHHash            // hash stack to create tick
	Recorder      POHRecorder          //
	Validators    []dataType.Validator // list validator to communicate, choose leader
	LeaderIndex   int                  // idx of leader in Validators
	HashPerSecond int                  //
	TickPerSecond int                  //
	TickPerSlot   int                  // these 3 variable use to sync hashrate between validators
	TimeOutTicks  int
	TickStart     int64 //
	TickEnd       int64 // use to throttle tick speed

}
