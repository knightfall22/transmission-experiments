package protocol

import (
	"log"
	"testing"
)

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

	log.Println(message)
}
