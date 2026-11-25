package rcon

import (
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	MaxConnections = 35
	authTimeout    = 5 * time.Second

	typeResponse = 0
	typeCommand  = 2
	typeAuthOK   = 2
	typeAuth     = 3
)

type Handler func(command string) string

type Server struct {
	log      *slog.Logger
	password string
	handle   Handler
	ln       net.Listener

	mu      sync.Mutex
	clients map[net.Conn]struct{}
	exec    sync.Mutex
	closed  bool
}

func Listen(log *slog.Logger, addr, password string, handle Handler) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s := &Server{log: log, password: password, handle: handle, ln: ln, clients: map[net.Conn]struct{}{}}
	go s.accept()
	return s, nil
}

func (s *Server) accept() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		if s.closed || len(s.clients) >= MaxConnections {
			s.mu.Unlock()
			_ = c.Close()
			continue
		}
		s.clients[c] = struct{}{}
		s.mu.Unlock()
		if tc, ok := c.(*net.TCPConn); ok {
			_ = tc.SetKeepAlive(true)
		}
		go s.serve(c)
	}
}

func (s *Server) serve(c net.Conn) {
	defer s.disconnect(c)
	authenticated := false
	_ = c.SetReadDeadline(time.Now().Add(authTimeout))
	for {
		requestID, kind, payload, err := readPacket(c)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				s.log.Debug("rcon connection error", "addr", c.RemoteAddr().String(), "err", err)
			}
			return
		}
		switch kind {
		case typeAuth:
			if authenticated {
				return
			}
			if payload == s.password {
				s.log.Info("Successful Rcon connection from: " + c.RemoteAddr().String())
				_ = writePacket(c, requestID, typeAuthOK, "")
				authenticated = true
				_ = c.SetReadDeadline(time.Time{})
			} else {
				_ = writePacket(c, -1, typeAuthOK, "")
				s.log.Info("Unsuccessful connection from: " + c.RemoteAddr().String() + " (wrong password)")
				return
			}
		case typeCommand:
			if !authenticated {
				return
			}
			if payload == "" {
				continue
			}
			s.exec.Lock()
			response := s.handle(strings.TrimLeft(payload, " \t\n\r\x00\x0b"))
			s.exec.Unlock()
			response = strings.ReplaceAll(strings.Trim(response, " \t\n\r\x00\x0b"), "\n", "\r\n")
			if err := writePacket(c, requestID, typeResponse, response); err != nil {
				return
			}
		}
	}
}

func (s *Server) disconnect(c net.Conn) {
	s.mu.Lock()
	delete(s.clients, c)
	s.mu.Unlock()
	addr := c.RemoteAddr().String()
	_ = c.Close()
	s.log.Info("Disconnected client: " + addr)
}

func readPacket(r io.Reader) (int32, int32, string, error) {
	var head [4]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return 0, 0, "", err
	}
	size := int32(binary.LittleEndian.Uint32(head[:]))
	if size < 0 || size > 65535 {
		return 0, 0, "", errors.New("packet with too-large length header")
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return 0, 0, "", err
	}
	if size < 8 {
		return 0, 0, "", errors.New("truncated packet")
	}
	requestID := int32(binary.LittleEndian.Uint32(buf[0:4]))
	kind := int32(binary.LittleEndian.Uint32(buf[4:8]))
	payload := ""
	if size > 10 {
		payload = string(buf[8 : size-2])
	}
	return requestID, kind, payload, nil
}

func writePacket(w io.Writer, requestID, kind int32, payload string) error {
	body := make([]byte, 0, 10+len(payload))
	body = binary.LittleEndian.AppendUint32(body, uint32(requestID))
	body = binary.LittleEndian.AppendUint32(body, uint32(kind))
	body = append(body, payload...)
	body = append(body, 0, 0)
	out := binary.LittleEndian.AppendUint32(make([]byte, 0, 4+len(body)), uint32(len(body)))
	_, err := w.Write(append(out, body...))
	return err
}

func (s *Server) Close() error {
	s.mu.Lock()
	s.closed = true
	clients := make([]net.Conn, 0, len(s.clients))
	for c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()
	err := s.ln.Close()
	for _, c := range clients {
		_ = c.Close()
	}
	return err
}

func (s *Server) Addr() net.Addr {
	return s.ln.Addr()
}
