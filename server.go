package gonami

import (
	"fmt"
	"log"
	"net"
	"path/filepath"
)

type Server struct {
	port             int
	encoder          Encoder
	TransfersChannel chan chan Progress
	localDirectory   string
}

type serverTransfer struct {
	c          Config
	progressCh chan Progress
	fn         string
	ld         string
	controlCh  chan controlMsg
}

type controlMsgType int

type controlMsg struct {
	msgType MessageType
	payload interface{}
}

func (st *serverTransfer) config() Config {
	return st.c
}

func (st *serverTransfer) updateProgress(progress Progress) {
	select {
	case st.progressCh <- progress:
		log.Println("Notifying progress listener")
	default:
		log.Println("No progess listener...")
	}
}

func (st *serverTransfer) filename() string {
	return st.fn
}

func (st *serverTransfer) localDirectory() string {
	return st.ld
}

func (st *serverTransfer) fullPath() string {
	return filepath.Join(st.localDirectory(), st.filename())
}

func newServerTransfer(progressCh chan Progress, localDirectory string) *serverTransfer {
	return &serverTransfer{progressCh: progressCh, ld: localDirectory}
}

func NewServer(encoder Encoder, port int, localDirectory string) *Server {
	tc := make(chan chan Progress)
	return &Server{port: port, encoder: encoder, TransfersChannel: tc, localDirectory: localDirectory}
}

func (s *Server) StartListening() {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		log.Fatal("Error listening:", err.Error())
	}
	// Close the listener when the application closes.
	defer l.Close()
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			log.Fatal("Error accepting: ", err.Error())
		}
		// Handle connections in a new goroutine.
		log.Println("Incoming connection accepted")
		ch := make(chan Progress)
		//non blocking send, in case the server doesn't care about
		//tracking progress
		select {
		case s.TransfersChannel <- ch:
			log.Println("Initializing progress listener")
		default:
			log.Println("No progess listener...")
		}
		go s.handleRequest(conn, ch)
	}
}

func (s *Server) handleRequest(conn net.Conn, ch chan Progress) {
	defer conn.Close()
	defer close(ch)
	st := newServerTransfer(ch, s.localDirectory)
	st.updateProgress(Progress{Type: HANDSHAKING, Message: "Accepted connection from: " + conn.RemoteAddr().String(), Percentage: 0})
	readPackets(conn, s.encoder, st, onVersionState)
	log.Println("Closing connection")
}
