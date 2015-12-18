package gonami

import (
	"log"
	"net"
)

const (
	secret     = "kitten"
	revision   = 20061025
	readBuffer = 100000
)

func xORSecret(b []byte, secret string) []byte {
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

func sendPacket(pkt *Packet, conn net.Conn, encoder Encoder) (int, error) {

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

func readPackets(conn net.Conn, e Encoder, t transfer, initialState stateFn) {
	inTransmission := true
	stateMachine := newStateMachine(initialState)
	for inTransmission {
		data := make([]byte, readBuffer)
		// Read the incoming connection into the buffer.
		numBytes, err := conn.Read(data)
		if err != nil {
			log.Println("Error reading bytes: " + err.Error())
			return
		}
		packet, err := e.Decode(data, numBytes)
		if err != nil {
			log.Println("Error decoding bytes: " + err.Error())
			return
		}
		inTransmission = stateMachine.transition(packet, e, conn, t)
	}
}
