package utils

import (
	"fmt"
	"log"
	"net"
	"time"

	protocol "github.com/knightfall22/transmission-experiments/Protocol"
)

func GetFirstOpenPort(address string, port int) int {
	var availablePort int
	for p := port; p-port < 200; p++ {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(address, fmt.Sprint(p)), 30*time.Second)

		if conn != nil {
			conn.Close()
			continue
		} else if err != nil {
			availablePort = p
		}

	}

	return availablePort
}

func PingServer(address string) error {
	log.Printf("pinging %s", address)
	c, err := net.DialTimeout("tcp", address, 300*time.Millisecond)
	if err != nil {
		log.Println(err)
		return err
	}

	msg := protocol.Message{ID: protocol.MessagePiece}
	_, err = c.Write(msg.Serialize())
	if err != nil {
		log.Println(err)
		return err
	}

	m, err := protocol.DeserializeMessageFromReader(c)
	if err != nil {
		log.Println(err)
		return err
	}

	if m.ID == protocol.MessagePong {
		return nil
	}

	return fmt.Errorf("no pong")
}
