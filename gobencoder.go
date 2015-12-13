package gonami

import (
	"bytes"
	"encoding/gob"
)

type GobEncoder struct{}

func NewGobEncoder() GobEncoder {
	gob.Register(Config{})
	gob.Register(Block{})
	gob.Register(Retransmit{})
	return GobEncoder{}
}

func (g GobEncoder) Encode(msg *Packet) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(msg); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (g GobEncoder) Decode(data []byte, numBytes int) (*Packet, error) {
	msg := Packet{}
	if err := gob.NewDecoder(bytes.NewReader(data[:numBytes])).Decode(&msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
