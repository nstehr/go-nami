package shared

import (
	"log"
	"net"

	"github.com/nstehr/go-nami/encoder"
	"github.com/nstehr/go-nami/message"
	"github.com/nstehr/go-nami/shared/transfer"
	"github.com/nstehr/go-nami/statemachine"
)

const (
	Secret   = "kitten"
	Revision = 20061025
)

func XORSecret(b []byte, secret string) []byte {
	var n int
	if len(secret) < len(b) {
		n = len(secret)
	} else {
		n = len(b)
	}
	r := make([]byte, n)
	for i := 0; i < n; i++ {
		r[i] = b[i] ^ secret[i]
	}

	return r
}

func SendPacket(pkt *message.Packet, conn net.Conn, encoder encoder.Encoder) (int, error) {

	b, err := encoder.Encode(pkt)

	if err != nil {
		return -1, err
	}
	numBytes, err := conn.Write(b)

	if err != nil {
		return -1, err
	}
	return numBytes, nil
}

func ReadPackets(conn net.Conn, e encoder.Encoder, ch chan transfer.Progress, initialState statemachine.StateFn) {
	inTransmission := true
	stateMachine := statemachine.NewStateMachine(initialState)
	for inTransmission {
		data := make([]byte, 8000)
		// Read the incoming connection into the buffer.
		numBytes, err := conn.Read(data)
		if err != nil {
			log.Println(err)
			return
		}
		packet, err := e.Decode(data, numBytes)
		if err != nil {
			log.Println(err)
			return
		}
		inTransmission = stateMachine.Transition(packet, e, conn, ch)
	}
}
