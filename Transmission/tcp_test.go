package transmission

import (
	"fmt"
	"net"
	"testing"
	"time"
)

// "9208"
func TestStart(t *testing.T) {
	var p Peer
	p.State = sender
	p.id, _ = generatePeerID(sender)
	err := p.Send(Options{FilePath: "C:/Users/Elite Gamer/Downloads/Video/Batman- The Court of Owls Full Story Motion Comic.mp4"})
	fmt.Println(err)
	var l Peer

	senderAddress := net.JoinHostPort(LOCAL_DEFAULT_ADDRESS, p.portStr)
	go func() {
		err := l.Listen(Options{SenderAddress: senderAddress, MaxPieceRetries: 4, DownloadFilePath: "./download_test"})
		fmt.Println("err", err)
	}()

	// go func() {
	// 	var l2 Peer
	// 	err := l2.Listen(Options{SenderAddress: senderAddress, MaxPieceRetries: 4})
	// 	fmt.Println("err", err)
	// }()

	time.Sleep(500000 * time.Millisecond)
	fmt.Println(err)
	fmt.Printf("file length: %d\n", p.Metadata.FileLength)
	// os.WriteFile("./g.pdf", buf, 0666)

	fmt.Println(p.Metadata.Checksum)
}

func TestStart2(t *testing.T) {
	// var p Peer
	// p.State = sender
	// p.id, _ = generatePeerID(sender)
	// err := p.Send(Options{FilePath: "../test_assets/TCP-IP.pdf"})
	// fmt.Println(err)

	var l Peer

	err := l.Listen(Options{MaxPieceRetries: 4})
	fmt.Println("err", err)

	time.Sleep(20 * time.Minute)

	// go func() {
	// 	var l2 Peer
	// 	buf, err := l2.Listen(Options{SenderAddress: senderAddress, MaxPieceRetries: 4})
	// 	fmt.Println("err", err)
	// 	fmt.Println(sha1.Sum(buf))
	// }()

	// go func() {
	// 	var l3 Peer
	// 	buf, err := l3.Listen(Options{SenderAddress: senderAddress, MaxPieceRetries: 4})
	// 	fmt.Println("err", err)
	// 	fmt.Println(sha1.Sum(buf))
	// }()

	// time.Sleep(500 * time.Millisecond)
	// fmt.Println(err)
	// fmt.Printf("file length: %d\n", p.Metadata.FileLength)
	// // os.WriteFile("./g.pdf", buf, 0666)

	// fmt.Println(p.Metadata.Checksum)
}

func TestListenerClose(t *testing.T) {
	var p Peer
	p.State = sender
	p.id, _ = generatePeerID(sender)
	err := p.Send(Options{FilePath: "../test_assets/TCP-IP.pdf"})
	if err != nil {
		t.Fatalf("an error has occurred: %v\n", err)
	}

	senderAddress := net.JoinHostPort(LOCAL_DEFAULT_ADDRESS, p.portStr)
	var l Peer
	l.SenderAddress = senderAddress
	l.id, _ = generatePeerID(receiver)

	conn, err := l.connectToSender()
	if err != nil {
		t.Fatalf("an error has occurred: %v\n", err)
	}

	err = l.listenerRelayHandshake(conn)
	if err != nil {
		t.Fatalf("an error has occurred: %v\n", err)
	}

	conn.Close()
	time.Sleep(1 * time.Second)

}
