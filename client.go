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

type clientTransfer struct {
	fn         string
	c          Config
	progressCh chan Progress
	filesize   int64
	ld         string
}

func (ct *clientTransfer) config() Config {
	return ct.c
}

func (ct *clientTransfer) updateProgress(progress Progress) {
	ct.progressCh <- progress
}

func (ct *clientTransfer) filename() string {
	return ct.fn
}

func (ct *clientTransfer) localDirectory() string {
	return ct.ld
}

func (ct *clientTransfer) fullPath() string {
	return filepath.Join(ct.localDirectory(), ct.filename())
}

func newClientTransfer(filename string, localDirectory string, c Config, progressCh chan Progress) *clientTransfer {
	return &clientTransfer{fn: filename, ld: localDirectory, c: c, progressCh: progressCh}
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
	ct := newClientTransfer(filename, localDirectory, config, ch)
	readPackets(conn, e, ct, onVersionConfirmedState)
}
