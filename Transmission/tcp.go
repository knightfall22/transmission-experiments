package transmission

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	protocol "github.com/knightfall22/transmission-experiments/Protocol"
	"github.com/knightfall22/transmission-experiments/Transmission/utils"
	"github.com/schollz/peerdiscovery"
)

type PeerState int8

func (p PeerState) String() string {
	switch p {
	case sender:
		return "sender"
	case receiver:
		return "receiver"
	case relay:
		return "relay"

	}
	return ""
}

const (
	sender PeerState = iota + 1
	receiver
	relay
)

type pieceWorker struct {
	index int
	piece [20]byte
}

type Peer struct {
	id      string
	State   PeerState
	Port    int
	portStr string

	mu sync.Mutex

	//Idea might need to be `net.IP`
	ExtenalIP string

	//Todo: this might change
	OpenFile *os.File
	Metadata *protocol.Metadata

	SenderAddress         string
	ConnectedSenderPort   string
	ConnectedSenderPortV6 string

	//Amount of times we can retry when some piece face a validation error
	//Default is 5
	//When max piece retries is 0 an error is thrown and the transfer is cancelled
	MaxPieceRetries int

	Listeners     []net.Conn
	ListenerLimit int
	Sender        net.Conn

	MulticastAddress string

	DownloadFilePath string

	//When a new listener joins it's incremented.
	// When a listener completes it decrements. When == 0, notifies the sender to close the file
	ToCommplete int

	shutdown chan struct{}

	//used internally
	selfConn net.Listener
	wg       sync.WaitGroup
}

type Options struct {
	SenderAddress    string
	FilePath         string
	MaxPieceRetries  int
	ListenerLimit    int
	MulticastAddress string
	DownloadFilePath string
}

// Idea change from method to function and allow options struct
// Create a sender peer, start a relay connect sender peer to relay
func (p *Peer) Send(opts Options) error {
	if p.Port == 0 {
		//Fetch available port
		p.Port = utils.GetFirstOpenPort(LOCAL_DEFAULT_ADDRESS, DEFAULT_PORT)
		p.portStr = fmt.Sprint(p.Port)
	}

	//Generate metadata from file
	meta, err := protocol.GenerateMetadata(opts.FilePath)
	if err != nil {
		return err
	}

	p.Metadata = meta
	p.MulticastAddress = opts.MulticastAddress

	if opts.ListenerLimit == 0 {
		p.ListenerLimit = 4
	}

	//Open file
	//Todo: Proper file close logic
	file, err := os.Open(opts.FilePath)
	if err != nil {
		return err
	}

	p.OpenFile = file

	p.dlog("starting sender server")

	go p.run(LOCAL_DEFAULT_ADDRESS)
	go p.broadcastOnLocalNetwork(false)
	go p.broadcastOnLocalNetwork(true)

	time.Sleep(500 * time.Millisecond)
	return nil
}

func (p *Peer) Start() error {
	id, err := generatePeerID(p.State)
	if err != nil {
		return err
	}

	fmt.Println(id)
	return nil
}

func (p *Peer) Listen(opts Options) (err error) {
	//Idea: accept relay from peer discovery or option
	p.State = receiver

	if opts.SenderAddress == "" {
		p.dlog("attempting to discover peers")

		var discoveries []peerdiscovery.Discovered
		var wg sync.WaitGroup
		var dmu sync.Mutex

		wg.Add(2)
		go func() {
			defer wg.Done()

			//Ipv4 discoveries
			ipv4Discoveries, err1 := peerdiscovery.Discover(peerdiscovery.Settings{
				Limit:            1,
				Payload:          []byte("ok"),
				TimeLimit:        500 * time.Millisecond,
				Delay:            20 * time.Millisecond,
				MulticastAddress: p.MulticastAddress,
			})

			if err == nil && len(ipv4Discoveries) > 0 {
				dmu.Lock()
				err = err1
				discoveries = append(discoveries, ipv4Discoveries...)
				dmu.Unlock()
			}

		}()
		go func() {
			defer wg.Done()

			//Ipv4 discoveries
			ipv4Discoveries, err1 := peerdiscovery.Discover(peerdiscovery.Settings{
				Limit:            1,
				Payload:          []byte("ok"),
				TimeLimit:        500 * time.Millisecond,
				Delay:            20 * time.Millisecond,
				MulticastAddress: p.MulticastAddress,
				IPVersion:        peerdiscovery.IPv6,
			})

			if err == nil && len(ipv4Discoveries) > 0 {
				dmu.Lock()
				err = err1
				discoveries = append(discoveries, ipv4Discoveries...)
				dmu.Unlock()
			}

		}()
		wg.Wait()

		if err != nil {
			return err
		}

		wasDiscovered := false

		if err == nil && len(discoveries) > 0 {
			p.dlog("all discovered peers %+v\n", discoveries)

			for i, discovered := range discoveries {
				if !bytes.HasPrefix(discovered.Payload, []byte("hello")) {
					p.dlog("skipping discovery %d", i)
					continue
				}

				port := string(bytes.TrimPrefix(discovered.Payload, []byte("hello")))

				if port == "" {
					continue
				}

				address := net.JoinHostPort(discovered.Address, port)
				err := utils.PingServer(address)
				if err == nil {
					p.SenderAddress = address
					wasDiscovered = true
					break
				}
			}
		}

		if !wasDiscovered {
			return fmt.Errorf("no peers found")
		}
	} else {
		p.SenderAddress = opts.SenderAddress
	}

	p.id, _ = generatePeerID(receiver)
	p.MaxPieceRetries = opts.MaxPieceRetries

	conn, err := p.connectToSender()
	if err != nil {
		return err
	}

	if err := p.listenerRelayHandshake(conn); err != nil {
		p.dlog("an error occurred sending relay handshake: %v\n", err)
		conn.Close()
		return err
	}

	if err := p.listenerRequestMetadata(conn); err != nil {
		p.dlog("an error occurred requesting metadata: %v\n", err)
		conn.Close()
		return err
	}

	//Create file from metadata information
	if opts.DownloadFilePath == "" {
		opts.DownloadFilePath = "./"
	}

	// Ensure the download directory exists
	if err := os.MkdirAll(opts.DownloadFilePath, 0755); err != nil {
		return err
	}

	p.DownloadFilePath = filepath.Join(opts.DownloadFilePath, p.Metadata.Name)

	file, err := os.OpenFile(p.DownloadFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	p.OpenFile = file

	workers := make(chan pieceWorker, len(p.Metadata.Pieces))
	result := make(chan protocol.PieceBlock)
	errChan := make(chan error, 1)

	for idx, piece := range p.Metadata.Pieces {
		workers <- pieceWorker{index: idx, piece: piece}
	}

	go func() {
		p.download(workers, conn, result, errChan)
	}()

	buf := make([]byte, 0, FLUSH_TRIGGER)
	done := 0

	for done < len(p.Metadata.Pieces) {
		select {
		case res := <-result:
			// Append piece data to rolling buffer
			buf = append(buf, res.Buf...)

			// Flush when buffer is full
			if len(buf) >= FLUSH_TRIGGER {
				if _, err := p.OpenFile.Write(buf); err != nil {
					return err
				}
				if err := p.OpenFile.Sync(); err != nil {
					return err
				}
				buf = buf[:0]
			}

		case err := <-errChan:
			return err
		}
		done++
	}

	// Final flush if buffer has leftover data
	if len(buf) > 0 {
		n := len(buf) - 1
		if _, err := p.OpenFile.Write(buf[:n]); err != nil {
			return err
		}
		if err := p.OpenFile.Sync(); err != nil {
			return err
		}
	}

	close(workers)
	conn.Close()
	return nil
}

func (p *Peer) connectToSender() (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", p.SenderAddress, 30*time.Second)
	if err != nil {
		return nil, err
	}

	p.dlog("connected to sender on port %s", conn.RemoteAddr().String())

	return conn, err
}

func (p *Peer) download(workers chan pieceWorker, conn net.Conn, result chan protocol.PieceBlock, errChan chan error) {
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		p.dlog("an error has occured while listening %v\n", err)
		errChan <- err
	}

	for work := range workers {
		p.dlog("requesting piece %d", work.index)
		_, err := conn.Write(requestPiece(work.index))
		if err != nil {
			p.dlog("an error has occured while listening %v\n", err)
			errChan <- err
		}

		//Expect to read a piece
		msg, err := protocol.DeserializeMessageFromReader(conn)
		if err != nil {
			p.dlog("an error has occured while listening %v\n", err)
			errChan <- err
		}
		p.dlog("received piece %d", work.index)

		if msg.ID != protocol.MessagePiece {
			p.dlog("message is not a piece")
			errChan <- err
		}

		resPiece, err := protocol.UnmarshallPiece(msg)
		if err != nil {
			p.dlog("an error has occured while listening %v\n", err)
			errChan <- err
		}

		if !p.verifyPiece(resPiece) {
			if p.MaxPieceRetries != 0 {
				p.dlog("piece at index %d does not match retrying....", resPiece.Index)
				workers <- work
				p.mu.Lock()
				p.MaxPieceRetries--
				p.mu.Unlock()
				continue
			}

			p.dlog("piece at index %d does not match", resPiece.Index)
			errChan <- fmt.Errorf("piece at index %d does not match", resPiece.Index)
		}

		result <- *resPiece
		conn.SetDeadline(time.Time{})
	}
}

func (p *Peer) run(host string) {

	network := "tcp"
	addr := net.JoinHostPort(host, p.portStr)
	if host != "" {
		ip := net.ParseIP(host)
		if ip == nil {
			var tcpIP *net.IPAddr
			tcpIP, err := net.ResolveIPAddr("ip", host)
			if err != nil {
				panic(err)
			}
			ip = tcpIP.IP
		}
		addr = net.JoinHostPort(ip.String(), p.portStr)
		if host != "" {
			if ip.To4() != nil {
				network = "tcp4"
			} else {
				network = "tcp6"
			}
		}
	}

	addr = strings.Replace(addr, "127.0.0.1", "0.0.0.0", 1)
	p.dlog("running sender server on %s", addr)

	l, err := net.Listen(network, addr)
	if err != nil {
		p.dlog(err.Error())
		panic(err)
	}

	p.mu.Lock()
	p.selfConn = l
	p.mu.Unlock()

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			conn, err := p.selfConn.Accept()
			if err != nil {
				select {
				case <-p.shutdown:
					p.dlog("closing server")
					return
				default:
					p.dlog(err.Error())
					return
				}
			}

			p.dlog("%s has connected", conn.RemoteAddr().String())

			p.wg.Add(1)
			go func(conn net.Conn) {
				defer func() {
					conn.Close()
					p.wg.Done()
					p.mu.Lock()
					for i, c := range p.Listeners {
						if c == conn {
							p.Listeners = append(p.Listeners[:i], p.Listeners[i+1:]...)
							break
						}
					}
					p.mu.Unlock()

					p.dlog("listener %s disconnected, remaining listeners: %d", conn.RemoteAddr(), len(p.Listeners))
				}()

				for {
					p.dlog("waiting for message from %s", conn.RemoteAddr())
					p.dlog("listener length: %d", len(p.Listeners))
					if err := p.messageProcessor(conn); err != nil {
						p.dlog("listener %s error or EOF: %v", conn.RemoteAddr(), err)
						return
					}
				}
			}(conn)

		}
	}()
}

func (p *Peer) broadcastOnLocalNetwork(useipv6 bool) {
	p.dlog("broadcasting on local network")
	// look for peers first
	settings := peerdiscovery.Settings{
		Limit:     -1,
		Payload:   []byte("hello" + p.portStr),
		Delay:     20 * time.Millisecond,
		TimeLimit: -1,
	}
	if useipv6 {
		settings.IPVersion = peerdiscovery.IPv6
	} else {
		settings.MulticastAddress = p.MulticastAddress
	}

	discoveries, err := peerdiscovery.Discover(settings)
	p.dlog("discoveries: %+v", discoveries)

	if err != nil {
		p.dlog("an error has occurred while broadcasting", err)
	}
}

// // Read an acknowledgement message from the relay
// func (p *Peer) senderRelayHandshake(conn net.Conn) error {
// 	p.dlog("Reading acknowledgment message")
// 	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
// 		return err
// 	}

// 	defer conn.SetReadDeadline(time.Time{})

// 	_, err := conn.Write(senderRelayHandshake())
// 	if err != nil {
// 		return err
// 	}

// 	msg, err := protocol.DeserializeMessageFromReader(conn)
// 	if err != nil {
// 		return err
// 	}

// 	if msg.ID == protocol.MessageSenderAcknowledgement {
// 		p.Relay = conn
// 		return nil
// 	} else {
// 		p.dlog("panicing relay acknowledgment not received")
// 		panic("supposed to receive relay acknowledgement")
// 	}
// }

// Read an acknowledgement message from the relay
func (p *Peer) listenerRelayHandshake(conn net.Conn) error {
	p.dlog("perform listener relay handshake message")
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return err
	}

	defer conn.SetReadDeadline(time.Time{})

	_, err := conn.Write(listenerRelayHandshake())
	if err != nil {
		return err
	}

	msg, err := protocol.DeserializeMessageFromReader(conn)
	if err != nil {
		return err
	}

	if msg.ID == protocol.MessageListenerAcknowledgement {
		p.Sender = conn
		return nil
	} else {
		p.dlog("panicing relay acknowledgment not received")
		panic("supposed to receive relay acknowledgement")
	}
}

// Read file metadata from relay
func (p *Peer) listenerRequestMetadata(conn net.Conn) error {
	p.dlog("perform request metadata handshake")
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return err
	}

	defer conn.SetReadDeadline(time.Time{})

	_, err := conn.Write(requestMetadata())
	if err != nil {
		return err
	}

	msg, err := protocol.DeserializeMessageFromReader(conn)
	if err != nil {
		return err
	}

	if msg.ID == protocol.MessageMetadata {
		p.mu.Lock()
		p.Metadata, err = protocol.UnmarshallMetadata(msg)
		if err != nil {
			return err
		}

		p.dlog("received metadata from relay")
		p.mu.Unlock()
		return nil
	} else {
		p.dlog("panicing relay acknowledgment not received")
		panic("supposed to receive relay acknowledgement")
	}
}

func (p *Peer) messageProcessor(conn net.Conn) error {
	msg, err := protocol.DeserializeMessageFromReader(conn)
	if err != nil {
		return err
	}

	switch msg.ID {
	case protocol.MessageSenderRelayHandshake:
		p.dlog("sender detected")
		p.mu.Lock()
		if p.Sender != nil {
			p.dlog("sender already exists, this shouldn't happen")
		}
		_, err := conn.Write(senderRelayAck())
		if err != nil {
			p.mu.Unlock()
			return err
		}
		p.Sender = conn
		p.mu.Unlock()

	case protocol.MessageListenerRelayHandshake:
		p.mu.Lock()
		p.dlog("listener detected")
		if len(p.Listeners) != p.ListenerLimit {
			_, err := conn.Write(senderListenerAck())
			if err != nil {
				p.mu.Unlock()
				return err
			}
			p.Listeners = append(p.Listeners, conn)
		}
		p.mu.Unlock()

	case protocol.MessagePing:
		_, err := conn.Write(sendPong())
		if err != nil {
			return err
		}

	case protocol.MessageRequestMetadata:
		msg, err := protocol.MarshallMetadata(p.Metadata)
		if err != nil {
			return err
		}
		_, err = conn.Write(msg.Serialize())
		if err != nil {
			return err
		}

	case protocol.MessageRequestPiece:
		p.dlog("%s has requested a piece", conn.RemoteAddr().String())

		idx := parsePieceRequest(msg.Payload)
		msg, err := protocol.MarshallPiece(p.OpenFile, idx)
		if err != nil {
			return err
		}

		_, err = conn.Write(msg.Serialize())
		if err != nil {
			return err
		}

		p.dlog("sent piece %d to relay: %s", idx, conn.RemoteAddr().String())
	}
	return nil
}

// Calculate the size of a piece
func (p *Peer) calculatePieceSize(index int) int {
	begin, end := p.calculateBoundsForPiece(index)
	return end - begin
}

// Given the piece length and file length. Find the boundary of a piece at an index
func (p *Peer) calculateBoundsForPiece(index int) (begin, end int) {
	begin = index * int(p.Metadata.PieceLength)
	end = begin + int(p.Metadata.PieceLength)

	if end > int(p.Metadata.PieceLength) {
		end = int(p.Metadata.FileLength)
	}

	return begin, end
}

func (p *Peer) verifyPiece(piece *protocol.PieceBlock) bool {
	hash := sha1.Sum(piece.Buf)

	return bytes.Equal(hash[:], p.Metadata.Pieces[piece.Index][:])
}

// dlog logs a debugging message if DebugCM > 0.
func (p *Peer) dlog(format string, args ...any) {
	format = fmt.Sprintf("[%s] ", p.id) + format
	log.Printf(format, args...)

}

// Generate peer ID from state
func generatePeerID(state PeerState) (string, error) {
	stateStr := state.String()

	byt := make([]byte, 5)
	_, err := rand.Read(byt)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s_%x", stateStr, byt), nil
}

func senderRelayHandshake() []byte {
	msg := protocol.Message{ID: protocol.MessageSenderRelayHandshake}
	return msg.Serialize()
}

func listenerRelayHandshake() []byte {
	msg := protocol.Message{ID: protocol.MessageListenerRelayHandshake}
	return msg.Serialize()
}

func senderRelayAck() []byte {
	msg := protocol.Message{ID: protocol.MessageSenderAcknowledgement}
	return msg.Serialize()
}

func senderListenerAck() []byte {
	msg := protocol.Message{ID: protocol.MessageListenerAcknowledgement}
	return msg.Serialize()
}

func requestMetadata() []byte {
	msg := protocol.Message{ID: protocol.MessageRequestMetadata}
	return msg.Serialize()
}

func requestPiece(index int) []byte {
	return protocol.RequestPiece(index)
}

func requestPieceAck() []byte {
	msg := protocol.Message{ID: protocol.MessagePieceAcknowledgement}
	return msg.Serialize()
}

func parsePieceRequest(byt []byte) int {
	index := int(binary.BigEndian.Uint32(byt[0:4]))

	return index
}

func sendPong() []byte {
	msg := protocol.Message{ID: protocol.MessagePong}

	return msg.Serialize()
}
