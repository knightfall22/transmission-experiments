package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

var formatPayload = []byte{0x0, 0x0, 0x0, 0xb, 0x65, 0x78, 0x61, 0x6d, 0x70,
	0x6c, 0x65, 0x2e, 0x74, 0x78, 0x74, 0x0, 0x0, 0x0, 0x4, 0x74, 0x65, 0x78,
	0x74, 0x0, 0x0, 0x0, 0x14, 0x12, 0x34, 0x56, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
	0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x4, 0x0, 0x0, 0x0, 0x0,
	0x0, 0x6, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x0, 0x0, 0x4, 0x0}

func TestFormatFileInfo(t *testing.T) {
	file := FileInfo{
		Name:        "example.txt",
		Type:        "text",
		Checksum:    [20]byte{0x12, 0x34, 0x56},
		PieceLength: 262144,
		Pieces:      "abcdef",
		FileLength:  1024,
	}

	message, err := FormatInfo(file)
	if err != nil {
		t.Fatalf("an error as occured while formatting file %v\n", err)
	}

	if !bytes.Equal(message.Payload, formatPayload) {
		t.Fatalf("invalid message payload")
	}

}

func TestFormatFileInfoSerialization(t *testing.T) {
	file := FileInfo{
		Name:        "example.txt",
		Type:        "text",
		Checksum:    [20]byte{0x12, 0x34, 0x56},
		PieceLength: 262144,
		Pieces:      "abcdef",
		FileLength:  1024,
	}

	message, err := FormatInfo(file)
	if err != nil {
		t.Fatalf("an error as occured while formatting file %v\n", err)
	}

	if !bytes.Equal(message.Payload, formatPayload) {
		t.Fatalf("invalid message payload")
	}

	serial := message.Serialize()

	var size uint32
	if err := binary.Read(bytes.NewReader(serial), binary.BigEndian, &size); err != nil {
		t.Fatalf("an error as occured while formatting file %v\n", err)
	}

	expectedSize := len(formatPayload) + 1
	if size != uint32(expectedSize) {
		t.Fatalf("invalid message size")
	}
}

func TestFormatFileInfoDeSerialization(t *testing.T) {
	file := FileInfo{
		Name:        "example.txt",
		Type:        "text",
		Checksum:    [20]byte{0x12, 0x34, 0x56},
		PieceLength: 262144,
		Pieces:      "abcdef",
		FileLength:  1024,
	}

	message, err := FormatInfo(file)
	if err != nil {
		t.Fatalf("an error as occured while formatting file %v\n", err)
	}

	if !bytes.Equal(message.Payload, formatPayload) {
		t.Fatalf("invalid message payload")
	}

	serial := message.Serialize()

	var size uint32
	if err := binary.Read(bytes.NewReader(serial), binary.BigEndian, &size); err != nil {
		t.Fatalf("an error as occured while formatting file %v\n", err)
	}

	expectedSize := len(formatPayload) + 1
	if size != uint32(expectedSize) {
		t.Fatalf("invalid message size")
	}

	msg, err := DeserializeMessage(serial)
	if err != nil {
		t.Fatalf("an error as occured while deserialing the message %v\n", err)
	}

	if msg.ID != message.ID {
		t.Fatalf("invalid message id")
	}
}

func TestFormatFileInfoParsing(t *testing.T) {
	file := FileInfo{
		Name:        "example.txt",
		Type:        "text",
		Checksum:    [20]byte{0x12, 0x34, 0x56},
		PieceLength: 262144,
		Pieces:      "abcdef",
		FileLength:  1024,
	}

	message, err := FormatInfo(file)
	if err != nil {
		t.Fatalf("an error as occured while formatting file %v\n", err)
	}

	if !bytes.Equal(message.Payload, formatPayload) {
		t.Fatalf("invalid message payload")
	}

	serial := message.Serialize()

	var size uint32
	if err := binary.Read(bytes.NewReader(serial), binary.BigEndian, &size); err != nil {
		t.Fatalf("an error as occured while formatting file %v\n", err)
	}

	expectedSize := len(formatPayload) + 1
	if size != uint32(expectedSize) {
		t.Fatalf("invalid message size")
	}

	msg, err := DeserializeMessage(serial)
	if err != nil {
		t.Fatalf("an error as occured while deserialing the message %v\n", err)
	}

	if msg.ID != message.ID {
		t.Fatalf("invalid message id")
	}

	newFile, err := ParseInfo(*message)
	if err != nil {
		t.Fatalf("an error as occured while parsing the message %v\n", err)
	}

	if file.Name != newFile.Name {
		t.Fatalf("invalid file name got %s wanted %s", newFile.Name, file.Name)
	}

	if file.Type != newFile.Type {
		t.Fatalf("invalid file type got %s wanted %s", newFile.Type, file.Type)
	}

	if file.Checksum != newFile.Checksum {
		t.Fatalf("invalid file checksum got %s wanted %s", newFile.Checksum, file.Checksum)
	}

	if file.PieceLength != newFile.PieceLength {
		t.Fatalf("invalid file piece length got %d wanted %d", newFile.PieceLength, file.PieceLength)
	}

	if file.Pieces != newFile.Pieces {
		t.Fatalf("invalid file pieces got %s wanted %s", newFile.Pieces, file.Pieces)
	}

	if file.FileLength != newFile.FileLength {
		t.Fatalf("invalid file length got %d wanted %d", newFile.FileLength, file.FileLength)
	}
}
