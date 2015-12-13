package gonami

type Encoder interface {
	Encode(msg *Packet) ([]byte, error)
	Decode(data []byte, numBytes int) (*Packet, error)
}
