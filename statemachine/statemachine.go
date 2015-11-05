package statemachine

import (
	"net"

	"github.com/nstehr/go-nami/encoder"
	"github.com/nstehr/go-nami/message"
	"github.com/nstehr/go-nami/shared/transfer"
)

type StateFn func(*message.Packet, encoder.Encoder, net.Conn, transfer.Transfer) StateFn

type StateMachine struct {
	currentState StateFn
}

func NewStateMachine(initialState StateFn) *StateMachine {
	return &StateMachine{currentState: initialState}
}

func (s *StateMachine) Transition(pkt *message.Packet, encoder encoder.Encoder, conn net.Conn, t transfer.Transfer) bool {
	s.currentState = s.currentState(pkt, encoder, conn, t)
	return s.currentState != nil
}
