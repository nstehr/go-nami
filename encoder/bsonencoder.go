package encoder

import (
	"reflect"
	"strings"

	"gopkg.in/mgo.v2/bson"

	"github.com/nstehr/go-nami/message"
	"github.com/nstehr/go-nami/shared/transfer"
)

//a bit hacked together, but seems to be better than the default
//gobencoder

type BsonEncoder struct{}

func (b BsonEncoder) Encode(msg *message.Packet) ([]byte, error) {
	data, err := bson.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (b BsonEncoder) Decode(data []byte, numBytes int) (*message.Packet, error) {
	msg := message.Packet{}
	err := bson.Unmarshal(data, &msg)
	if err != nil {
		return nil, err
	}
	switch msg.Type {
	case message.GET_FILE:
		if payload, ok := msg.Payload.(bson.M); ok {
			c := transfer.Config{}
			fillStruct(payload, &c)
			msg.Payload = c
		}
	case message.DATA:
		if payload, ok := msg.Payload.(bson.M); ok {
			b := message.Block{}
			fillStruct(payload, &b)
			msg.Payload = b
		}
	case message.RETRANSMIT:
		if payload, ok := msg.Payload.(bson.M); ok {
			b := message.Retransmit{}
			fillStruct(payload, &b)
			pBlockNums := payload["blocknums"].([]interface{})
			blockNums := make([]int, len(pBlockNums))
			for i, block := range pBlockNums {
				blockNums[i] = block.(int)
			}
			b.BlockNums = blockNums
			msg.Payload = b
		}
	}
	return &msg, nil
}
func fillStruct(data map[string]interface{}, result interface{}) {
	t := reflect.ValueOf(result).Elem()
	typeOfT := t.Type()
	for i := 0; i < t.NumField(); i++ {
		fieldName := typeOfT.Field(i).Name
		v := strings.ToLower(fieldName)
		if mVal, ok := data[v]; ok {
			sVal := t.FieldByName(fieldName)
			if fieldName == "Type" {
				sVal.SetInt(int64(reflect.ValueOf(mVal).Interface().(int)))
			} else if fieldName == "BlockNums" {
				continue
			} else {
				sVal.Set(reflect.ValueOf(mVal))
			}

		}
	}
}
