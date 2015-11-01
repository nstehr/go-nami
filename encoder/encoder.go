package encoder

import (
	"github.com/nstehr/go-nami/message"
)

type Encoder interface {
	Encode(msg *message.Packet) ([]byte, error)
	Decode(data []byte, numBytes int) (*message.Packet, error)
}
