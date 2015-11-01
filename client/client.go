package client

import (
	"log"
	"net"

	"github.com/nstehr/go-nami/encoder"
	"github.com/nstehr/go-nami/message"
	"github.com/nstehr/go-nami/shared"
	"github.com/nstehr/go-nami/shared/transfer"
)

type Client struct {
	encoder encoder.Encoder
}

func NewClient(encoder encoder.Encoder) *Client {
	return &Client{encoder: encoder}
}

func (c *Client) GetFile(filename string, serverAddr string) <-chan transfer.Progress {
	ch := make(chan transfer.Progress)
	go getFile(filename, serverAddr, c.encoder, ch)
	return ch
}

func getFile(filename string, serverAddr string, e encoder.Encoder, ch chan transfer.Progress) {
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
	shared.ReadPackets(conn, e, ch, onVersionConfirmed)
}
