package server

import (
	"log"
	"math"
	"net"
	"os"
	"time"

	"github.com/nstehr/go-nami/encoder"
	"github.com/nstehr/go-nami/message"
	"github.com/nstehr/go-nami/shared"
)

func sendFile(client string, e encoder.Encoder, t *ServerTransfer) {
	listeningAddr, err := net.ResolveUDPAddr("udp", client)
	if err != nil {
		log.Println("Error resolving: " + client)
		return
	}
	file, err := os.Open(t.FullPath()) // For read access.
	if err != nil {
		log.Println("Error opening file: " + err.Error())
	}

	stat, err := file.Stat()
	if err != nil {
		log.Println("Error getting file stats: " + err.Error())
	}
	filesize := stat.Size()

	blockSize := t.Config().BlockSize
	transferRate := float64(t.Config().TransferRate) * 0.125 //get the transfer in bytes per second

	blockRate := math.Floor(transferRate / float64(blockSize)) //how many blocks we can send in one second
	numBlocks := math.Ceil(float64(filesize) / float64(blockSize))

	//we can send more blocks per second than we need send
	//so only send the number of blocks
	if blockRate > numBlocks {
		blockRate = numBlocks
	}
	blockIndex := 0

	conn, err := net.DialUDP("udp", nil, listeningAddr)
	ticker := time.NewTicker(time.Second * 1)
	for {
		select {
		case <-ticker.C:
			for i := float64(0); i < blockRate; i++ {
				sendDataPkt(file, blockSize, blockIndex, conn, e, message.ORIGINAL)
				blockIndex++
			}
			if blockIndex == int(numBlocks) {
				ticker.Stop()
			}
		case msg := <-t.controlCh:
			if msg.msgType == message.DONE {
				return
			}
			if msg.msgType == message.RETRANSMIT {
				rt := msg.payload.(message.Retransmit)
				blocks := rt.BlockNums
				for _, block := range blocks {
					sendDataPkt(file, blockSize, block, conn, e, message.RETRANSMITTED)
				}

			}
			if msg.msgType == message.ERROR_RATE {

			}
		}
	}
}

func sendDataPkt(file *os.File, blockSize int64, blockIndex int, conn net.Conn, e encoder.Encoder, blockType message.BlockType) {
	bytes := make([]byte, blockSize)
	numBytes, _ := file.ReadAt(bytes, int64(blockIndex)*blockSize)
	//if we are at the end of the file, chances are the bytes left will
	//be less than blockSize, so adjust
	if int64(numBytes) < blockSize {
		bytes = bytes[0:numBytes]
	}
	block := message.Block{Number: blockIndex, Data: bytes, Type: blockType}
	outPkt := &message.Packet{Type: message.DATA, Payload: block}
	_, err := shared.SendPacket(outPkt, conn, e)
	if err != nil {
		log.Println("Error sending packet: " + err.Error())
	}
}
