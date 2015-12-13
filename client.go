package gonami

import (
	"log"
	"net"
	"path/filepath"
)

type Client struct {
	encoder        Encoder
	config         Config
	localDirectory string
}

type ClientTransfer struct {
	filename       string
	config         Config
	progressCh     chan Progress
	filesize       int64
	localDirectory string
}

func (ct *ClientTransfer) Config() Config {
	return ct.config
}

func (ct *ClientTransfer) UpdateProgress(progress Progress) {
	ct.progressCh <- progress
}

func (ct *ClientTransfer) Filename() string {
	return ct.filename
}

func (ct *ClientTransfer) LocalDirectory() string {
	return ct.localDirectory
}

func (ct *ClientTransfer) FullPath() string {
	return filepath.Join(ct.LocalDirectory(), ct.Filename())
}

func NewClientTransfer(filename string, localDirectory string, config Config, progressCh chan Progress) *ClientTransfer {
	return &ClientTransfer{filename: filename, localDirectory: localDirectory, config: config, progressCh: progressCh}
}

func NewClient(localDirectory string, config Config, encoder Encoder) *Client {
	return &Client{encoder: encoder, config: config, localDirectory: localDirectory}
}

func (c *Client) GetFile(filename string, serverAddr string) <-chan Progress {
	ch := make(chan Progress)
	go getFile(filename, c.localDirectory, serverAddr, c.encoder, c.config, ch)
	return ch
}

func getFile(filename string, localDirectory string, serverAddr string, e Encoder, config Config, ch chan Progress) {
	defer close(ch)
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		errMsg := "Error establishing connection: " + err.Error()
		log.Println(errMsg)
		ch <- Progress{Type: ERROR, Message: errMsg, Percentage: 0}
		return
	}
	defer conn.Close()
	//start everything off with sending our version number
	pkt := Packet{Type: REV, Payload: revision}
	ch <- Progress{Type: HANDSHAKING, Message: "Sending client version", Percentage: 0}
	b, _ := e.Encode(&pkt)
	conn.Write(b)
	ct := NewClientTransfer(filename, localDirectory, config, ch)
	readPackets(conn, e, ct, onVersionConfirmedState)
}
