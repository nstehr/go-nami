package gonami

import (
	"log"
	"math"
	"net"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/willf/bitset"
)

const (
	retransmitIteration = 50
	retransmitTimeDelta = 320 * time.Millisecond
	readTimeout         = 2 * time.Second
)

func handleDownload(e Encoder, controlConn net.Conn, dataConn *net.UDPConn, t *ClientTransfer) {
	var wg sync.WaitGroup

	numBlocks := int(math.Ceil(float64(t.filesize) / float64(t.Config().BlockSize)))

	bs := bitset.New(uint(numBlocks))
	defer dataConn.Close()
	fo, err := os.Create(t.FullPath())
	defer fo.Close()
	if err != nil {
		errMsg := "Error opening file: " + err.Error()
		log.Println(errMsg)
		t.UpdateProgress(Progress{Type: ERROR, Message: errMsg, Percentage: 0})
	}
	fileWriter := make(chan Block)
	defer close(fileWriter)

	//handles writing the blocks to the file
	wg.Add(1)
	go func() {
		defer wg.Done()
		for block := range fileWriter {
			writeData(block.Data, block.Number*t.Config().BlockSize, fo)
		}
	}()

	expectedBlock := 0
	gaplessToBlock := 0
	missedBlocks := 0
	receivedBlocks := 0

	lastRetransmitTime := time.Now()
	var retransmitBlocks []int

	buf := make([]byte, t.Config().BlockSize+500)
	dataConn.SetReadDeadline(time.Now().Add(readTimeout))

	for {
		n, _, err := dataConn.ReadFromUDP(buf)
		dataConn.SetReadDeadline(time.Now().Add(readTimeout))
		if err != nil {
			if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
				//we timedout on a read, but don't have all the data
				//so send a retransmit and try again
				restart := false
				if len(retransmitBlocks) <= 0 {
					retransmitBlocks = insertRetransmitBlock(retransmitBlocks, gaplessToBlock+1)
					restart = true
				}
				requestRetransmit(retransmitBlocks, bs, controlConn, e, restart)
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
		block := pkt.Payload.(Block)
		//send the block to be written
		fileWriter <- block
		bs.Set(uint(block.Number))
		receivedBlocks++
		if block.Number > expectedBlock {
			if (len(retransmitBlocks) + (block.Number - expectedBlock)) > t.Config().MaxMissedLength {
				requestRetransmit(retransmitBlocks, bs, controlConn, e, true)
				retransmitBlocks = []int{}
			} else {
				for i := expectedBlock; i < block.Number; i++ {
					retransmitBlocks = insertRetransmitBlock(retransmitBlocks, i)
				}
			}
			missedBlocks = missedBlocks + (block.Number - expectedBlock)
		}
		//if we have received all the blocks, we are done!
		if int(bs.Count()) == numBlocks {
			pkt := Packet{Type: DONE}
			data, _ := e.Encode(&pkt)
			controlConn.Write(data)
			t.UpdateProgress(Progress{Type: TRANSFERRING, Message: "Finalizing file", Percentage: 1})
			wg.Wait()
			return
		}
		//we will be expecting the next block number
		//in case of restart: these resent blocks are labeled original as well
		if block.Type == ORIGINAL {
			expectedBlock = block.Number + 1
		}
		//keeps track of the point up to where we have received all the blocks
		//with no missing blocks in between
		for bs.Test(uint(gaplessToBlock+1)) && gaplessToBlock < numBlocks {
			gaplessToBlock++
		}
		//if we meet our retransmit criteria, send message to server
		if shouldRetransmit(bs.Count(), lastRetransmitTime) {
			//send the error rate
			sendErrorRate(receivedBlocks, missedBlocks, controlConn, e)
			//request the retransmit
			requestRetransmit(retransmitBlocks, bs, controlConn, e, false)
			retransmitBlocks = []int{}
			lastRetransmitTime = time.Now()
			missedBlocks = 0
			receivedBlocks = 0
		}
		//finally, update progress
		t.UpdateProgress(Progress{Type: TRANSFERRING, Message: "Downloading...", Percentage: float64(bs.Count()) / float64(numBlocks)})
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
		blocks = append(blocks, block)

		return blocks
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

func requestRetransmit(blocks []int, bs *bitset.BitSet, conn net.Conn, e Encoder, isRestart bool) {
	if len(blocks) <= 0 {
		return
	}
	var missingBlocks []int
	if isRestart {
		missingBlocks = blocks[0:1]
	} else {
		for _, b := range blocks {
			if !bs.Test(uint(b)) {
				missingBlocks = append(missingBlocks, b)
			}
		}
	}

	payload := Retransmit{IsRestart: isRestart, BlockNums: missingBlocks}
	pkt := Packet{Type: RETRANSMIT, Payload: payload}
	_, err := sendPacket(&pkt, conn, e)
	if err != nil {
		log.Println("Error sending retransmit blocks: " + err.Error())
	}
}

func sendErrorRate(receivedBlocks int, missedBlocks int, conn net.Conn, e Encoder) {
	percent := float64(missedBlocks) / float64(missedBlocks+receivedBlocks)
	pkt := Packet{Type: ERROR_RATE, Payload: percent}
	_, err := sendPacket(&pkt, conn, e)
	if err != nil {
		log.Println("Error sending error rate: " + err.Error())
	}
}

func writeData(data []byte, offset int, fo *os.File) {
	_, err := fo.WriteAt(data, int64(offset))
	if err != nil {
		log.Println("Error writing to file: " + err.Error())
	}
}
