package client

import (
	"log"
	"net"
	"path/filepath"

	"github.com/nstehr/go-nami/encoder"
	"github.com/nstehr/go-nami/message"
	"github.com/nstehr/go-nami/shared"
	"github.com/nstehr/go-nami/shared/transfer"
)

type Client struct {
	encoder        encoder.Encoder
	config         transfer.Config
	localDirectory string
}

type ClientTransfer struct {
	filename       string
	config         transfer.Config
	progressCh     chan transfer.Progress
	filesize       int64
	localDirectory string
}

func (ct *ClientTransfer) Config() transfer.Config {
	return ct.config
}

func (ct *ClientTransfer) UpdateProgress(progress transfer.Progress) {
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

func NewClientTransfer(filename string, localDirectory string, config transfer.Config, progressCh chan transfer.Progress) *ClientTransfer {
	return &ClientTransfer{filename: filename, localDirectory: localDirectory, config: config, progressCh: progressCh}
}

func NewClient(localDirectory string, config transfer.Config, encoder encoder.Encoder) *Client {
	return &Client{encoder: encoder, config: config, localDirectory: localDirectory}
}

func (c *Client) GetFile(filename string, serverAddr string) <-chan transfer.Progress {
	ch := make(chan transfer.Progress)
	go getFile(filename, c.localDirectory, serverAddr, c.encoder, c.config, ch)
	return ch
}

func getFile(filename string, localDirectory string, serverAddr string, e encoder.Encoder, config transfer.Config, ch chan transfer.Progress) {
	defer close(ch)
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		log.Println("Error establishing connection: " + err.Error())
		return
	}
	defer conn.Close()
	//start everything off with sending our version number
	pkt := message.Packet{Type: message.REV, Payload: shared.Revision}
	b, _ := e.Encode(&pkt)
	conn.Write(b)
	ct := NewClientTransfer(filename, localDirectory, config, ch)
	shared.ReadPackets(conn, e, ct, onVersionConfirmedState)
}
