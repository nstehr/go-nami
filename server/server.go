package server

import (
	"fmt"
	"log"
	"net"
	"path/filepath"

	"github.com/nstehr/go-nami/encoder"
	"github.com/nstehr/go-nami/shared"
	"github.com/nstehr/go-nami/shared/transfer"
)

type Server struct {
	port             int
	encoder          encoder.Encoder
	TransfersChannel chan chan transfer.Progress
	localDirectory   string
}

type ServerTransfer struct {
	config         transfer.Config
	progressCh     chan transfer.Progress
	filename       string
	localDirectory string
}

func (st *ServerTransfer) Config() transfer.Config {
	return st.config
}

func (st *ServerTransfer) UpdateProgress(progress transfer.Progress) {
	select {
	case st.progressCh <- progress:
		log.Println("Notifying progress listener")
	default:
		log.Println("No progess listener...")
	}
}

func (st *ServerTransfer) Filename() string {
	return st.filename
}

func (st *ServerTransfer) LocalDirectory() string {
	return st.localDirectory
}

func (st *ServerTransfer) FullPath() string {
	return filepath.Join(st.LocalDirectory(), st.Filename())
}

func NewServerTransfer(progressCh chan transfer.Progress, localDirectory string) *ServerTransfer {
	return &ServerTransfer{progressCh: progressCh, localDirectory: localDirectory}
}

func NewServer(encoder encoder.Encoder, port int, localDirectory string) *Server {
	tc := make(chan chan transfer.Progress)
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
		ch := make(chan transfer.Progress)
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

func (s *Server) handleRequest(conn net.Conn, ch chan transfer.Progress) {
	defer conn.Close()
	defer close(ch)
	st := NewServerTransfer(ch, s.localDirectory)
	shared.ReadPackets(conn, s.encoder, st, onVersionState)
	log.Println("Closing connection")
}
