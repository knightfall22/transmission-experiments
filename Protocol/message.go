package protocol

import (
	"bytes"
	"encoding/binary"
)

// Todo: Change this location
type FileInfo struct {
	Name        string
	Type        string
	Checksum    [20]byte
	PieceLength int
	Pieces      string
	FileLength  int
}

// Defines the messaging format for peer to peer communication
type MessageCode int8

const (
	MessageFileInfo MessageCode = iota + 1
)

type Message struct {
	ID MessageCode

	//Contains a sequence of bytes in this format <length><data>. Length is a type of uint16.
	//It is important to decode the payload in order lest you get bad data.
	//All variable length type except for ints have a prefix
	Payload []byte
}

// Serializes message into <size><id><payload>.
// <size> is the size of id + payload
func (m *Message) Serialize() []byte {
	length := len(m.Payload) + 1

	bytSlice := make([]byte, length+4)

	//Add size to return slice
	binary.LittleEndian.PutUint32(bytSlice[0:4], uint32(length))

	//Add message Id
	bytSlice[4] = byte(m.ID)

	copy(bytSlice[5:], m.Payload)

	return bytSlice
}

func FormatInfo(file FileInfo) (*Message, error) {
	message := Message{ID: MessageFileInfo}

	var buf bytes.Buffer

	err := binary.Write(&buf, binary.LittleEndian, uint16(len(file.Name)))
	if err != nil {
		return nil, err
	}

	err = binary.Write(&buf, binary.LittleEndian, file.Name)
	if err != nil {
		return nil, err
	}

	err = binary.Write(&buf, binary.LittleEndian, uint16(len(file.Type)))
	if err != nil {
		return nil, err
	}

	err = binary.Write(&buf, binary.LittleEndian, file.Type)
	if err != nil {
		return nil, err
	}

	err = binary.Write(&buf, binary.LittleEndian, uint16(len(file.Checksum)))
	if err != nil {
		return nil, err
	}

	err = binary.Write(&buf, binary.LittleEndian, file.Checksum)
	if err != nil {
		return nil, err
	}

	err = binary.Write(&buf, binary.LittleEndian, uint16(file.PieceLength))
	if err != nil {
		return nil, err
	}

	err = binary.Write(&buf, binary.LittleEndian, uint16(len(file.Pieces)))
	if err != nil {
		return nil, err
	}

	err = binary.Write(&buf, binary.LittleEndian, file.Pieces)
	if err != nil {
		return nil, err
	}

	err = binary.Write(&buf, binary.LittleEndian, uint16(file.FileLength))
	if err != nil {
		return nil, err
	}

	message.Payload = buf.Bytes()

	return &message, nil
}
