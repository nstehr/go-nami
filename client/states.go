package client

import (
	"crypto/md5"
	"fmt"
	"log"
	"net"

	"github.com/nstehr/go-nami/encoder"
	"github.com/nstehr/go-nami/message"
	"github.com/nstehr/go-nami/shared"
	"github.com/nstehr/go-nami/shared/transfer"
	"github.com/nstehr/go-nami/statemachine"
)

func onVersionConfirmedState(pkt *message.Packet, e encoder.Encoder, conn net.Conn, t transfer.Transfer) statemachine.StateFn {
	if pkt.Type != message.AUTH {
		log.Println("Expecting AUTH, did not receive it")
		return nil
	}
	b, ok := pkt.Payload.([]byte)
	if !ok {
		log.Println("Incorrect payload type")
		return nil
	}
	t.UpdateProgress(transfer.Progress{Type: transfer.HANDSHAKING, Message: "Version correct, Authenticating", Percentage: 0.25})
	x := shared.XORSecret(b, shared.Secret)
	//and then MD5 hash it
	hasher := md5.New()
	outPkt := message.Packet{Type: message.AUTH, Payload: hasher.Sum(x)}
	_, err := shared.SendPacket(&outPkt, conn, e)
	if err != nil {
		log.Println("Error sending AUTH packet: " + err.Error())
		return nil
	}
	return onAuthenticatedState
}

func onAuthenticatedState(pkt *message.Packet, e encoder.Encoder, conn net.Conn, t transfer.Transfer) statemachine.StateFn {
	if pkt.Type != message.AUTH {
		log.Println("Expecting AUTH, did not receive it")
		return nil
	}
	authenticated, ok := pkt.Payload.([]byte)
	if !ok {
		log.Println("Incorrect payload type")
		return nil
	}
	if authenticated[0] != 000 {
		log.Println("Authentication failed")
		t.UpdateProgress(transfer.Progress{Type: transfer.ERROR, Message: "Authentication failed.", Percentage: 0})
	}
	t.UpdateProgress(transfer.Progress{Type: transfer.HANDSHAKING, Message: "Authenticated. Validating file with server", Percentage: 0.50})
	return sendFilenameState(pkt, e, conn, t)
}

func sendFilenameState(pkt *message.Packet, e encoder.Encoder, conn net.Conn, t transfer.Transfer) statemachine.StateFn {
	filename := t.Filename()
	outPkt := message.Packet{Type: message.GET_FILE, Payload: filename}
	_, err := shared.SendPacket(&outPkt, conn, e)
	if err != nil {
		log.Println("Error sending GET_FILE packet: " + err.Error())
		return nil
	}
	return onFilenameValidationState
}

func onFilenameValidationState(pkt *message.Packet, e encoder.Encoder, conn net.Conn, t transfer.Transfer) statemachine.StateFn {
	if pkt.Type != message.GET_FILE {
		log.Println("Expecting GET_FILE, did not receive it")
		return nil
	}
	payload, ok := pkt.Payload.([]byte)
	if !ok {
		log.Println("Incorrect payload type")
		return nil
	}
	if payload[0] != 000 {
		log.Println("problem accessing file on server")
		t.UpdateProgress(transfer.Progress{Type: transfer.ERROR, Message: "Problem accessing file on server", Percentage: 0})
		return nil
	}
	t.UpdateProgress(transfer.Progress{Type: transfer.HANDSHAKING, Message: "File exists, sending configuration", Percentage: 0.75})
	return sendTransferConfigState(pkt, e, conn, t)
}

func sendTransferConfigState(pkt *message.Packet, e encoder.Encoder, conn net.Conn, t transfer.Transfer) statemachine.StateFn {
	config := t.Config()
	outPkt := message.Packet{Type: message.GET_FILE, Payload: config}
	_, err := shared.SendPacket(&outPkt, conn, e)
	if err != nil {
		log.Println("Error sending GET_FILE packet: " + err.Error())
		return nil
	}
	return acceptFileSizeState
}

func acceptFileSizeState(pkt *message.Packet, e encoder.Encoder, conn net.Conn, t transfer.Transfer) statemachine.StateFn {
	if pkt.Type != message.GET_FILE {
		log.Println("Expecting GET_FILE, did not receive it")
		return nil
	}
	payload, ok := pkt.Payload.(int64)
	t.(*ClientTransfer).filesize = payload
	if !ok {
		log.Println("Incorrect payload type")
		return nil
	}
	serverConn, err := getUDPServerConn()
	if err != nil {
		errMsg := "Error starting listening connection: " + err.Error()
		log.Println(errMsg)
		t.UpdateProgress(transfer.Progress{Type: transfer.ERROR, Message: errMsg, Percentage: 0})
		return nil
	}
	listeningPort := serverConn.LocalAddr().(*net.UDPAddr).Port
	outPkt := message.Packet{Type: message.GET_FILE, Payload: listeningPort}
	_, err = shared.SendPacket(&outPkt, conn, e)
	if err != nil {
		log.Println("Error sending GET_FILE packet: " + err.Error())
		return nil
	}
	t.UpdateProgress(transfer.Progress{Type: transfer.HANDSHAKING, Message: "Handshaking complete. Starting Download", Percentage: 1})
	go handleDownload(e, conn, serverConn, t.(*ClientTransfer))
	return transferDoneState
}
func transferDoneState(pkt *message.Packet, e encoder.Encoder, conn net.Conn, t transfer.Transfer) statemachine.StateFn {
	return nil
}

func getUDPServerConn() (*net.UDPConn, error) {
	serverAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", 0))
	if err != nil {
		return nil, err
	}

	serverConn, err := net.ListenUDP("udp", serverAddr)
	if err != nil {
		return nil, err
	}
	return serverConn, nil
}
