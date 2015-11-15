package encoder

import (
	"bytes"
	"encoding/gob"

	"github.com/nstehr/go-nami/message"
	"github.com/nstehr/go-nami/shared/transfer"
)

type GobEncoder struct{}

func NewGobEncoder() GobEncoder {
	gob.Register(transfer.Config{})
	gob.Register(message.Block{})
	gob.Register(message.Retransmit{})
	return GobEncoder{}
}

func (g GobEncoder) Encode(msg *message.Packet) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(msg); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (g GobEncoder) Decode(data []byte, numBytes int) (*message.Packet, error) {
	msg := message.Packet{}
	if err := gob.NewDecoder(bytes.NewReader(data[:numBytes])).Decode(&msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
