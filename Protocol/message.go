package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// Todo: Change this location
type FileInfo struct {
	Name        string
	Type        string
	Checksum    [20]byte
	PieceLength int32
	Pieces      string
	FileLength  int32
}

// Defines the messaging format for peer to peer communication
type MessageCode int8

const (
	MessageFileInfo MessageCode = iota + 1
)

type Message struct {
	ID MessageCode

	//Contains a sequence of bytes in this format <length><data>. Length is a type of uint32.
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
	binary.BigEndian.PutUint32(bytSlice[0:4], uint32(length))

	//Add message Id
	bytSlice[4] = byte(m.ID)

	copy(bytSlice[5:], m.Payload)

	return bytSlice
}

func DeserializeMessage(message []byte) (*Message, error) {
	buf := bytes.NewReader(message)

	//Fetch Size
	size := make([]byte, 4)
	_, err := io.ReadFull(buf, size)
	if err != nil && err != io.EOF {
		return nil, err
	}

	msgLength := int32(binary.BigEndian.Uint32(size))

	payload := make([]byte, msgLength)
	_, err = io.ReadFull(buf, payload)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return &Message{
		ID:      MessageCode(payload[0]),
		Payload: payload[1:],
	}, nil
}

func FormatInfo(file FileInfo) (*Message, error) {
	message := Message{ID: MessageFileInfo}

	var buf bytes.Buffer

	err := writeString(&buf, file.Name)
	if err != nil {
		return nil, err
	}

	err = writeString(&buf, file.Type)
	if err != nil {
		return nil, err
	}

	err = binary.Write(&buf, binary.BigEndian, uint32(len(file.Checksum)))
	if err != nil {
		return nil, err
	}

	err = binary.Write(&buf, binary.BigEndian, file.Checksum)
	if err != nil {
		return nil, err
	}

	err = binary.Write(&buf, binary.BigEndian, uint32(file.PieceLength))
	if err != nil {
		return nil, err
	}

	err = writeString(&buf, file.Pieces)
	if err != nil {
		return nil, err
	}

	err = binary.Write(&buf, binary.BigEndian, uint32(file.FileLength))
	if err != nil {
		return nil, err
	}

	message.Payload = buf.Bytes()

	return &message, nil
}

func ParseInfo(message Message) (*FileInfo, error) {
	buf := bytes.NewReader(message.Payload)
	var length uint32

	//Extract name
	err := binary.Read(buf, binary.BigEndian, &length)
	if err != nil {
		return nil, err
	}

	name := make([]byte, length)
	if _, err := buf.Read(name); err != nil {
		return nil, err
	}

	//Extract Type
	err = binary.Read(buf, binary.BigEndian, &length)
	if err != nil {
		return nil, err
	}

	mimetype := make([]byte, length)
	if _, err := buf.Read(mimetype); err != nil {
		return nil, err
	}

	//Extract checksum
	err = binary.Read(buf, binary.BigEndian, &length)
	if err != nil {
		return nil, err
	}

	checksum := make([]byte, length)
	if _, err := buf.Read(checksum); err != nil {
		return nil, err
	}

	var checksumArr [20]byte
	copy(checksumArr[:], checksum)

	//Extract Piecelength
	var pieceLength int32
	err = binary.Read(buf, binary.BigEndian, &pieceLength)
	if err != nil {
		return nil, err
	}

	//Extract pieces
	err = binary.Read(buf, binary.BigEndian, &length)
	if err != nil {
		return nil, err
	}

	pieces := make([]byte, length)
	if _, err := buf.Read(pieces); err != nil {
		return nil, err
	}

	//Extract Piecelength
	var fileLength int32
	err = binary.Read(buf, binary.BigEndian, &fileLength)
	if err != nil {
		return nil, err
	}

	return &FileInfo{
		Name:        string(name),
		Type:        string(mimetype),
		Checksum:    checksumArr,
		PieceLength: pieceLength,
		Pieces:      string(pieces),
		FileLength:  fileLength,
	}, nil
}

func writeString(buf *bytes.Buffer, s string) error {
	b := []byte(s)

	err := binary.Write(buf, binary.BigEndian, uint32(len(b)))
	if err != nil {
		return err
	}
	n, err := buf.Write(b)
	if err != nil {
		return err
	}

	fmt.Println("num", n)
	return nil
}
