package client

import (
	"crypto/md5"
	"log"
	"net"

	"github.com/nstehr/go-nami/encoder"
	"github.com/nstehr/go-nami/message"
	"github.com/nstehr/go-nami/shared"
	"github.com/nstehr/go-nami/shared/transfer"
	"github.com/nstehr/go-nami/statemachine"
)

func onVersionConfirmed(pkt *message.Packet, e encoder.Encoder, conn net.Conn, progressCh chan transfer.Progress) statemachine.StateFn {
	if pkt.Type != message.AUTH {
		log.Println("Expecting AUTH, did not receive it")
		return nil
	}
	b, ok := pkt.Payload.([]byte)
	if !ok {
		log.Println("Incorrect payload type")
		return nil
	}
	log.Println("Version confirmed, generating AUTH token")
	x := shared.XORSecret(b, shared.Secret)
	//and then MD5 hash it
	hasher := md5.New()
	outPkt := message.Packet{Type: message.AUTH, Payload: hasher.Sum(x)}
	b, _ = e.Encode(&outPkt)
	conn.Write(b)
	return onAuthenticated
}

func onAuthenticated(pkt *message.Packet, e encoder.Encoder, conn net.Conn, progressCh chan transfer.Progress) statemachine.StateFn {
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
	}
	log.Println("Authentication successful")
	return nil
}
