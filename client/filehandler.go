package client

import (
	"log"
	"math"
	"net"
	"os"
	"sort"
	"time"

	"github.com/nstehr/go-nami/encoder"
	"github.com/nstehr/go-nami/message"
	"github.com/nstehr/go-nami/shared"
	"github.com/willf/bitset"
)

const (
	retransmitIteration = 50
	retransmitTimeDelta = 320 * time.Millisecond
	readTimeout         = 2 * time.Second
)

func handleDownload(e encoder.Encoder, controlConn net.Conn, dataConn *net.UDPConn, t *ClientTransfer) {
	numBlocks := math.Ceil(float64(t.filesize) / float64(t.Config().BlockSize))
	log.Println(numBlocks)
	bs := bitset.New(uint(numBlocks))
	defer dataConn.Close()
	fo, err := os.Create(t.FullPath())
	defer fo.Close()
	if err != nil {
		log.Println("Error opening file: " + err.Error())
	}
	expectedBlock := 0
	gaplessToBlock := uint(0)
	lastRetransmitTime := time.Now()
	var retransmitBlocks []int

	buf := make([]byte, t.Config().BlockSize+500)
	dataConn.SetReadDeadline(time.Now().Add(readTimeout))

	for {
		n, _, err := dataConn.ReadFromUDP(buf)
		if err != nil {
			if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
				//we timedout on a read, but don't have all the data
				//so send a retransmit and try again

				//if retransmit blocks hits 0, but we know there are still missing blocks
				//use the gaplessToBlock value to fill in the blanks
				if len(retransmitBlocks) == 0 && float64(bs.Count()) < numBlocks {
					for i := gaplessToBlock + 1; float64(i) < numBlocks; i++ {
						retransmitBlocks = insertRetransmitBlock(retransmitBlocks, int(i))
					}
				}
				requestRetransmit(retransmitBlocks, bs, controlConn, e)
				retransmitBlocks = []int{}
				dataConn.SetReadDeadline(time.Now().Add(readTimeout))
				continue
			} else {
				log.Println("Error reading from socket: " + err.Error())
				return
			}

		}
		pkt, err := e.Decode(buf, n)
		if err != nil {
			log.Println(err)
			return
		}
		//write the block to file and build out the list of blocks
		//to retransmit
		block := pkt.Payload.(message.Block)
		writeData(block.Data, int64(block.Number)*t.Config().BlockSize, fo)
		bs.Set(uint(block.Number))
		if block.Number > expectedBlock {
			for i := expectedBlock; i < block.Number; i++ {
				retransmitBlocks = insertRetransmitBlock(retransmitBlocks, i)
			}
		}
		//if we have received all the blocks, we are done!
		if float64(bs.Count()) == numBlocks {
			pkt := message.Packet{Type: message.DONE}
			data, _ := e.Encode(&pkt)
			controlConn.Write(data)
			return
		}
		//we will be expecting the next block number
		//in case of restart: these resent blocks are labeled original as well
		if block.Type == message.ORIGINAL {
			expectedBlock = block.Number + 1
		}
		//keeps track of the point up to where we have received all the blocks
		//with no missing blocks in between
		for bs.Test(gaplessToBlock+1) && float64(gaplessToBlock) < numBlocks {
			gaplessToBlock++
		}
		//finally, if we meet our retransmit criteria, send message to server
		if shouldRetransmit(bs.Count(), lastRetransmitTime) {
			requestRetransmit(retransmitBlocks, bs, controlConn, e)
			retransmitBlocks = []int{}
			lastRetransmitTime = time.Now()
		}
	}
}

func shouldRetransmit(numBlocks uint, lastRetransmitTime time.Time) bool {
	now := time.Now()
	delta := now.Sub(lastRetransmitTime)
	if numBlocks%retransmitIteration == 0 && delta > retransmitTimeDelta {
		return true
	}
	return false
}

func insertRetransmitBlock(blocks []int, block int) []int {
	if len(blocks) == 0 {
		return append(blocks, block)
	}
	i := sort.Search(len(blocks), func(i int) bool { return blocks[i] >= block })
	// block is not present in data,
	// but i is the index where it would be inserted.
	if !(i < len(blocks) && blocks[i] == block) {
		newBlocks := make([]int, len(blocks)+1)
		copy(newBlocks, blocks)

		for j := i; j < len(blocks); j++ {
			newBlocks[j+1] = blocks[j]
		}

		newBlocks[i] = block
		blocks = newBlocks
	}
	return blocks
}

func requestRetransmit(blocks []int, bs *bitset.BitSet, conn net.Conn, e encoder.Encoder) {
	var missingBlocks []int
	for _, b := range blocks {
		if !bs.Test(uint(b)) {
			missingBlocks = append(missingBlocks, b)
		}
	}
	payload := message.Retransmit{IsRestart: false, BlockNums: missingBlocks}
	pkt := message.Packet{Type: message.RETRANSMIT, Payload: payload}
	_, err := shared.SendPacket(&pkt, conn, e)
	if err != nil {
		log.Println("Error sending retransmit blocks: " + err.Error())
	}
}

func writeData(data []byte, offset int64, fo *os.File) {
	_, err := fo.WriteAt(data, offset)
	if err != nil {
		log.Println("Error writing to file: " + err.Error())
	}
}