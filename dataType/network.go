package dataType

type Header struct {
	Id            string `json:"id"`
	Type          string `json:"type"` // request or respone
	From          string `json:"from"`
	Command       string `json:"command"`
	StatusCode    int    `json:"status_code"`
	Time          string `json:"time"` // for remove out of date messsage
	TotalReceived int    `json:"totalReceived"`
	TotalPackage  int    `json:"totalPackage"`
}

type Message struct {
	Header Header `json:"header"`
	Body   string `json:"body"`
}
