package message

type MessageType int

const (
	AUTH MessageType = iota
	REV
	GET_FILE
	DATA
	RETRANSMIT
	ERROR_RATE
	DONE
)

type Packet struct {
	Type    MessageType
	Payload interface{}
}

type BlockType int

const (
	ORIGINAL BlockType = iota
	RETRANSMITTED
)

type Block struct {
	Number int64
	Data   []byte
	Type   BlockType
}

type Retransmit struct {
	IsRestart bool
	BlockNums []int64
}
