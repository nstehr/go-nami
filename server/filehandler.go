package server

import (
	"log"
	"math"
	"net"
	"os"
	"sync"
	"time"

	"github.com/nstehr/go-nami/encoder"
	"github.com/nstehr/go-nami/message"
	"github.com/nstehr/go-nami/shared"
	"github.com/nstehr/go-nami/shared/transfer"
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
	log.Println(filesize)
	blockSize := t.Config().BlockSize
	transferRate := float64(t.Config().TransferRate) * 0.125 //get the transfer in bytes per second

	blockRate := int(math.Floor(transferRate / float64(blockSize))) //how many blocks we can send in one second
	numBlocks := int(math.Ceil(float64(filesize) / float64(blockSize)))

	conn, err := net.DialUDP("udp", nil, listeningAddr)

	sendPacketCh := make(chan *message.Packet)
	blockRateCh := make(chan float64)
	doneCh := make(chan bool)
	canStopRetransmit := make(chan chan bool)

	defer close(sendPacketCh)
	defer close(blockRateCh)
	defer close(doneCh)
	defer close(canStopRetransmit)

	go func() {
		packetSender(blockRate, conn, e, sendPacketCh, blockRateCh, doneCh)
	}()

	//send the inital set of packets
	go func() {
		for i := 0; i < numBlocks; i++ {
			sendDataPkt(file, blockSize, i, sendPacketCh, message.ORIGINAL)
		}
	}()
	//listen for commands messages
	canStop := false
	increaseCount := 0
	var wg sync.WaitGroup
	for {
		select {
		case msg := <-t.controlCh:
			if msg.msgType == message.DONE {
				doneCh <- true
				canStop = true
				wg.Wait()
				return
			}
			if msg.msgType == message.RETRANSMIT {
				rt := msg.payload.(message.Retransmit)
				blocks := rt.BlockNums
				wg.Add(1)
				go func() {
					defer wg.Done()
					canStopResponseCh := make(chan bool)
					if !rt.IsRestart {
						for _, block := range blocks {
							//this is a bit of a hack, but the transfer can potentially
							//be done, but since this is in the a goroutine it won't know
							//with out asking (or being told..)
							if canStopSendingPackets(canStopResponseCh, canStopRetransmit) {
								return
							}
							sendDataPkt(file, blockSize, block, sendPacketCh, message.RETRANSMITTED)
						}

					} else {
						startBlock := blocks[0]
						for i := startBlock; i < numBlocks; i++ {
							if canStopSendingPackets(canStopResponseCh, canStopRetransmit) {
								return
							}
							sendDataPkt(file, blockSize, i, sendPacketCh, message.ORIGINAL)
						}
					}
				}()
			}
			if msg.msgType == message.ERROR_RATE {
				errorRate := msg.payload.(float64)
				updateSendRate(errorRate, &increaseCount, t.Config(), blockRateCh)
			}
		case ch := <-canStopRetransmit:
			ch <- canStop
		}
	}

}

func sendDataPkt(file *os.File, blockSize int, blockIndex int, packetCh chan *message.Packet, blockType message.BlockType) {
	bytes := make([]byte, blockSize)
	numBytes, _ := file.ReadAt(bytes, int64(blockIndex*blockSize))
	//if we are at the end of the file, chances are the bytes left will
	//be less than blockSize, so adjust
	if numBytes < blockSize {
		bytes = bytes[0:numBytes]
	}
	block := message.Block{Number: blockIndex, Data: bytes, Type: blockType}
	outPkt := &message.Packet{Type: message.DATA, Payload: block}
	packetCh <- outPkt
}

func updateSendRate(errorRate float64, increaseCount *int, config transfer.Config, blockRateCh chan float64) {
	targetErrorRate := float64(config.ErrorRate) / float64(10000)
	increaseRate := 0.25
	consecutiveIncrease := 15
	if errorRate > targetErrorRate {
		percent := float64(config.SlowerNum) / float64(config.SlowerDen)
		blockRateCh <- percent
		log.Println("Decreasing....")
	}
	if errorRate < increaseRate {
		*increaseCount++
		if *increaseCount > consecutiveIncrease {
			percent := float64(config.FasterNum) / float64(config.FasterDen)
			blockRateCh <- percent
			*increaseCount = 0
			log.Println("Increasing....")
		}
	}

}

func canStopSendingPackets(canStopResponseCh chan bool, canStopRetransmit chan chan bool) bool {
	canStopRetransmit <- canStopResponseCh
	canStopSending := <-canStopResponseCh
	return canStopSending
}

func packetSender(initialBlockRate int, conn net.Conn, e encoder.Encoder, packetCh chan *message.Packet, blockRateCh chan float64, doneCh chan bool) {
	blockRate := initialBlockRate
	rate := time.Second / time.Duration(blockRate)
	throttle := time.NewTicker(rate)
	for {
		select {
		case packet := <-packetCh:
			if packet != nil {
				<-throttle.C
				_, err := shared.SendPacket(packet, conn, e)
				if err != nil {
					log.Println("Error sending packet: " + err.Error())
				}

			}
		case newRatePercent := <-blockRateCh:
			blockRate = int(math.Ceil(float64(blockRate) / newRatePercent))
			throttle.Stop()
			throttle = time.NewTicker(time.Second / time.Duration(blockRate))
		case <-doneCh:
			throttle.Stop()
			return
		}
	}
}
