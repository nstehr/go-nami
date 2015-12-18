package gonami

import (
	"crypto/md5"
	"fmt"
	"log"
	"net"
)

func onVersionConfirmedState(pkt *Packet, e Encoder, conn net.Conn, t transfer) stateFn {
	if pkt.Type != AUTH {
		log.Println("Expecting AUTH, did not receive it")
		return nil
	}
	b, ok := pkt.Payload.([]byte)
	if !ok {
		log.Println("Incorrect payload type")
		return nil
	}
	t.updateProgress(Progress{Type: HANDSHAKING, Message: "Version correct, Authenticating", Percentage: 0.25})
	x := xORSecret(b, secret)
	//and then MD5 hash it
	hasher := md5.New()
	outPkt := Packet{Type: AUTH, Payload: hasher.Sum(x)}
	_, err := sendPacket(&outPkt, conn, e)
	if err != nil {
		log.Println("Error sending AUTH packet: " + err.Error())
		return nil
	}
	return onAuthenticatedState
}

func onAuthenticatedState(pkt *Packet, e Encoder, conn net.Conn, t transfer) stateFn {
	if pkt.Type != AUTH {
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
		t.updateProgress(Progress{Type: ERROR, Message: "Authentication failed.", Percentage: 0})
	}
	t.updateProgress(Progress{Type: HANDSHAKING, Message: "Authenticated. Validating file with server", Percentage: 0.50})
	return sendFilenameState(pkt, e, conn, t)
}

func sendFilenameState(pkt *Packet, e Encoder, conn net.Conn, t transfer) stateFn {
	filename := t.filename()
	outPkt := Packet{Type: GET_FILE, Payload: filename}
	_, err := sendPacket(&outPkt, conn, e)
	if err != nil {
		log.Println("Error sending GET_FILE packet: " + err.Error())
		return nil
	}
	return onFilenameValidationState
}

func onFilenameValidationState(pkt *Packet, e Encoder, conn net.Conn, t transfer) stateFn {
	if pkt.Type != GET_FILE {
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
		t.updateProgress(Progress{Type: ERROR, Message: "Problem accessing file on server", Percentage: 0})
		return nil
	}
	t.updateProgress(Progress{Type: HANDSHAKING, Message: "File exists, sending configuration", Percentage: 0.75})
	return sendTransferConfigState(pkt, e, conn, t)
}

func sendTransferConfigState(pkt *Packet, e Encoder, conn net.Conn, t transfer) stateFn {
	config := t.config()
	outPkt := Packet{Type: GET_FILE, Payload: config}
	_, err := sendPacket(&outPkt, conn, e)
	if err != nil {
		log.Println("Error sending GET_FILE packet: " + err.Error())
		return nil
	}
	return acceptFileSizeState
}

func acceptFileSizeState(pkt *Packet, e Encoder, conn net.Conn, t transfer) stateFn {
	if pkt.Type != GET_FILE {
		log.Println("Expecting GET_FILE, did not receive it")
		return nil
	}
	payload, ok := pkt.Payload.(int64)
	t.(*clientTransfer).filesize = payload
	if !ok {
		log.Println("Incorrect payload type")
		return nil
	}
	serverConn, err := getUDPServerConn()
	if err != nil {
		errMsg := "Error starting listening connection: " + err.Error()
		log.Println(errMsg)
		t.updateProgress(Progress{Type: ERROR, Message: errMsg, Percentage: 0})
		return nil
	}
	listeningPort := serverConn.LocalAddr().(*net.UDPAddr).Port
	outPkt := Packet{Type: GET_FILE, Payload: listeningPort}
	_, err = sendPacket(&outPkt, conn, e)
	if err != nil {
		log.Println("Error sending GET_FILE packet: " + err.Error())
		return nil
	}
	t.updateProgress(Progress{Type: HANDSHAKING, Message: "Handshaking complete. Starting Download", Percentage: 1})
	go handleDownload(e, conn, serverConn, t.(*clientTransfer))
	return transferDoneState
}
func transferDoneState(pkt *Packet, e Encoder, conn net.Conn, t transfer) stateFn {
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
