package server

import (
	"crypto/md5"
	"crypto/rand"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/nstehr/go-nami/encoder"
	"github.com/nstehr/go-nami/message"
	"github.com/nstehr/go-nami/shared"
	"github.com/nstehr/go-nami/shared/transfer"
	"github.com/nstehr/go-nami/statemachine"
)

func onVersionState(pkt *message.Packet, e encoder.Encoder, conn net.Conn, t transfer.Transfer) statemachine.StateFn {
	log.Println("Comparing revisions")
	t.UpdateProgress(transfer.Progress{Type: transfer.HANDSHAKING, Message: "Comparing versions", Percentage: 0.25})
	if pkt.Type != message.REV {
		log.Println("Expecting REV, did not receive it")
		return nil
	}
	revision, ok := pkt.Payload.(int)
	if !ok {
		log.Println("Incorrect payload type")
		return nil
	}

	if revision != shared.Revision {
		log.Println("protocol revisions do not match")
		t.UpdateProgress(transfer.Progress{Type: transfer.ERROR, Message: "Versions do not match", Percentage: 0})
		return nil
	}
	return onBeginAuthState(pkt, e, conn, t)
}

func onBeginAuthState(pkt *message.Packet, e encoder.Encoder, conn net.Conn, t transfer.Transfer) statemachine.StateFn {
	log.Println("Generating auth token")
	//on connection, generate random bytes and send to the client
	random := generateRandomBytes()
	outPkt := &message.Packet{Type: message.AUTH, Payload: random}
	_, err := shared.SendPacket(outPkt, conn, e)
	if err != nil {
		log.Println("Error sending AUTH token: " + err.Error())
		return nil
	}
	//use a closure to capture the value of the randomly generated bytes
	authenticateStateWrapper := func(pkt1 *message.Packet, e1 encoder.Encoder, conn1 net.Conn, t1 transfer.Transfer) statemachine.StateFn {
		return authenticateClientState(pkt1, e1, conn1, t1, random)
	}
	return authenticateStateWrapper
}

func authenticateClientState(pkt *message.Packet, e encoder.Encoder, conn net.Conn, t transfer.Transfer, randomBytes []byte) statemachine.StateFn {
	log.Println("Authenticating client")
	if pkt.Type != message.AUTH {
		log.Println("Expecting AUTH, did not receive it")
		return nil
	}
	//get the bytes the client sent over
	b, ok := pkt.Payload.([]byte)
	if !ok {
		log.Println("Incorrect payload type")
		return nil
	}
	//XOR our generated bytes with the secret
	xORd := shared.XORSecret(randomBytes, shared.Secret)
	//and then MD5 hash to compare with client bytes
	hasher := md5.New()
	hashed := hasher.Sum(xORd)
	if len(hashed) != len(b) {
		log.Println("Authentication failed")
		t.UpdateProgress(transfer.Progress{Type: transfer.ERROR, Message: "Authentication failed", Percentage: 0})
		return nil
	}
	//compare the client bytes to our bytes to
	//see if the client is authenticated
	for i := 0; i < len(hashed); i++ {
		if hashed[i] != b[i] {
			log.Println("Authentication failed")
			t.UpdateProgress(transfer.Progress{Type: transfer.ERROR, Message: "Authentication failed", Percentage: 0})
			return nil
		}
	}
	t.UpdateProgress(transfer.Progress{Type: transfer.HANDSHAKING, Message: "Authentication Successful", Percentage: 0.50})
	outPkt := &message.Packet{Type: message.AUTH, Payload: []byte{000}}
	_, err := shared.SendPacket(outPkt, conn, e)
	if err != nil {
		log.Println("Error sending AUTH token: " + err.Error())
		return nil
	}
	return validateFilenameState
}

func validateFilenameState(pkt *message.Packet, e encoder.Encoder, conn net.Conn, t transfer.Transfer) statemachine.StateFn {
	if pkt.Type != message.GET_FILE {
		log.Println("Expecting GET_FILE, did not receive it")
		return nil
	}
	filename, ok := pkt.Payload.(string)
	if !ok {
		log.Println("Incorrect payload type")
		return nil
	}
	payload := []byte{000}
	fullPath := filepath.Join(t.LocalDirectory(), filename)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		msg := "no such file or directory: " + fullPath
		log.Println(msg)
		t.UpdateProgress(transfer.Progress{Type: transfer.ERROR, Message: msg, Percentage: 0})
		payload = []byte{001}
	}
	outPkt := &message.Packet{Type: message.GET_FILE, Payload: payload}
	_, err := shared.SendPacket(outPkt, conn, e)
	if err != nil {
		log.Println("Error sending GET_FILE: " + err.Error())
		return nil
	}
	//set the filename into the ServerTransfer
	t.(*ServerTransfer).filename = filename
	t.UpdateProgress(transfer.Progress{Type: transfer.HANDSHAKING, Message: "File found", Percentage: 0.75})
	return receiveConfigState
}

func receiveConfigState(pkt *message.Packet, e encoder.Encoder, conn net.Conn, t transfer.Transfer) statemachine.StateFn {
	if pkt.Type != message.GET_FILE {
		log.Println("Expecting GET_FILE, did not receive it")
		return nil
	}
	config, ok := pkt.Payload.(transfer.Config)
	if !ok {
		log.Println("Incorrect payload type")
		return nil
	}
	//send the filesize
	fullPath := t.FullPath()
	info, err := os.Stat(fullPath)
	if err != nil {
		log.Println("Error reading file: " + err.Error())
		return nil
	}
	filesize := info.Size()
	outPkt := &message.Packet{Type: message.GET_FILE, Payload: filesize}
	_, err = shared.SendPacket(outPkt, conn, e)
	if err != nil {
		log.Println("Error sending GET_FILE: " + err.Error())
		return nil
	}
	//save the config
	t.(*ServerTransfer).config = config
	t.UpdateProgress(transfer.Progress{Type: transfer.HANDSHAKING, Message: "Configuration received. Handshaking complete", Percentage: 1})

	return acceptListeningPortState
}

func acceptListeningPortState(pkt *message.Packet, e encoder.Encoder, conn net.Conn, t transfer.Transfer) statemachine.StateFn {
	port := pkt.Payload.(int)
	ip := conn.RemoteAddr().(*net.TCPAddr).IP.String()
	client := fmt.Sprintf("%s:%d", ip, port)
	t.(*ServerTransfer).controlCh = make(chan controlMsg)
	go sendFile(client, e, t.(*ServerTransfer))
	t.UpdateProgress(transfer.Progress{Type: transfer.TRANSFERRING, Message: "Starting transfer", Percentage: 0})
	return transferingState
}

func transferingState(pkt *message.Packet, e encoder.Encoder, conn net.Conn, t transfer.Transfer) statemachine.StateFn {

	switch pkt.Type {
	case message.RETRANSMIT:
		rt, ok := pkt.Payload.(message.Retransmit)
		if !ok {
			log.Println("Incorrect payload type")
			return nil
		}
		t.(*ServerTransfer).controlCh <- controlMsg{msgType: message.RETRANSMIT, payload: rt}
		return transferingState
	case message.ERROR_RATE:
		errorRate, ok := pkt.Payload.(float64)
		if !ok {
			log.Println("Incorrect payload type")
			return nil
		}
		t.(*ServerTransfer).controlCh <- controlMsg{msgType: message.ERROR_RATE, payload: errorRate}
	case message.DONE:
		t.(*ServerTransfer).controlCh <- controlMsg{msgType: message.DONE}
		data, _ := e.Encode(pkt)
		conn.Write(data)
		t.UpdateProgress(transfer.Progress{Type: transfer.TRANSFERRING, Message: "Transfer Complete", Percentage: 1})
		return nil
	}
	return transferingState
}

func generateRandomBytes() []byte {
	size := 64
	rb := make([]byte, size)
	_, err := rand.Read(rb)
	if err != nil {
		log.Println(err)
	}

	return rb
}
