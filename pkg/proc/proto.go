package proc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

type ProtoReader struct {
	c net.Conn
}

func NewProtoReader(c net.Conn) *ProtoReader {
	return &ProtoReader{c}
}

type MessageType byte

const (
	MessageTypeCat = MessageType(42)
)

func (m MessageType) String() string {
	switch m {
	case MessageTypeCat:
		return "CAT"
	}
	return "UNKNOWN"
}

const maxMsgLen = 1 << 20

func (r *ProtoReader) ReadPacket() (MessageType, []byte, error) {
	msgLenBuf := make([]byte, 8)
	_, err := io.ReadFull(r.c, msgLenBuf)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read params: %w", err)
	}

	dataLen := binary.BigEndian.Uint64(msgLenBuf)

	if dataLen > maxMsgLen {
		return 0, nil, fmt.Errorf("message too big")
	}

	data := make([]byte, dataLen)
	_, err = io.ReadFull(r.c, data)
	if err != nil {
		return 0, nil, err
	}

	msgType := MessageType(data[0])
	return msgType, data, nil
}

func GetCatName(b []byte) string {
	buff := bytes.NewBufferString("")

	for i := 0; i < len(b); i++ {
		if b[i] == 0 {
			break
		}
		buff.WriteByte(b[i])
	}

	return buff.String()
}

func ConstructMessage(name string) []byte {

	bt := []byte{
		byte(MessageTypeCat),
	}
	bt = append(bt, []byte(name)...)
	bt = append(bt, 0)
	ln := len(bt)

	bs := make([]byte, 8)
	binary.BigEndian.PutUint64(bs, uint64(ln))
	return append(bs, bt...)
}
