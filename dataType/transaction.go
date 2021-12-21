package dataType

type Transaction struct {
	Hash      string `json:"hash"`
	From      string `json:"from"`
	To        string `json:"to"`
	LastHash  string `json:"lastHash"`
	Amount    int    `json:"amount"`
	Sign      string `json:"sign"`
	BlockHash string `json:"blockHash"`
}
