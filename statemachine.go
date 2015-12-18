package gonami

import (
	"net"
)

type stateFn func(*Packet, Encoder, net.Conn, transfer) stateFn

type stateMachine struct {
	currentState stateFn
}

func newStateMachine(initialState stateFn) *stateMachine {
	return &stateMachine{currentState: initialState}
}

func (s *stateMachine) transition(pkt *Packet, encoder Encoder, conn net.Conn, t transfer) bool {
	s.currentState = s.currentState(pkt, encoder, conn, t)
	return s.currentState != nil
}
