package melody

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// List of errors, which can be returned from the Session methods
var (
	errWriteToClosedSession error = errors.New("tried to write to a closed session")
	errBufferIsFull         error = errors.New("session message buffer is full")
	errSessionAlreadyClosed error = errors.New("session is already closed")
)

// Session wrapper around websocket connections.
type Session struct {
	Request *http.Request
	Conn    *websocket.Conn
	Melody  *Melody
	rwmutex *sync.RWMutex
	keys    map[string]interface{}
	output  chan *envelope
	open    bool
}

// Deadline always returns that there is no deadline (ok==false),
func (s *Session) Deadline() (deadline time.Time, ok bool) {
	return
}

// Done always returns nil (chan which will wait forever),
func (s *Session) Done() <-chan struct{} {
	return nil
}

// Err always returns nil
func (s *Session) Err() error {
	return nil
}

// Value returns the value associated with this context for key, or nil
func (s *Session) Value(key interface{}) interface{} {
	if key == nil {
		return nil
	}
	if keyStr, ok := key.(string); ok {
		val, _ := s.Get(keyStr)
		return val
	}
	return nil
}

func (s *Session) writeMessage(message *envelope) {
	if s.closed() {
		s.Melody.errorHandler(s, errWriteToClosedSession)
		return
	}

	select {
	case s.output <- message:
	default:
		s.Melody.errorHandler(s, errBufferIsFull)
	}
}

func (s *Session) writeRaw(message *envelope) error {
	if s.closed() {
		return errWriteToClosedSession
	}

	s.Conn.SetWriteDeadline(time.Now().Add(s.Melody.Config.WriteWait))
	err := s.Conn.WriteMessage(message.t, message.msg)

	if err != nil {
		return err
	}

	return nil
}

func (s *Session) closed() (b bool) {
	s.rwmutex.RLock()
	b = !s.open
	s.rwmutex.RUnlock()
	return
}

func (s *Session) close() {
	if !s.closed() {
		s.rwmutex.Lock()
		close(s.output)
		s.open = false
		s.rwmutex.Unlock()
		s.Conn.Close()
	}
}

func (s *Session) ping() {
	s.writeRaw(&envelope{t: websocket.PingMessage, msg: []byte{}})
}

func (s *Session) writePump() {
	ticker := time.NewTicker(s.Melody.Config.PingPeriod)
	defer ticker.Stop()

loop:
	for {
		select {
		case msg, ok := <-s.output:
			if !ok {
				break loop
			}

			err := s.writeRaw(msg)

			if err != nil {
				s.Melody.errorHandler(s, err)
				break loop
			}

			if msg.t == websocket.CloseMessage {
				break loop
			}

			if msg.t == websocket.TextMessage {
				s.Melody.messageSentHandler(s, msg.msg)
			}

			if msg.t == websocket.BinaryMessage {
				s.Melody.messageSentHandlerBinary(s, msg.msg)
			}
		case <-ticker.C:
			s.ping()
		}
	}
}

func (s *Session) readPump() {
	s.Conn.SetReadLimit(s.Melody.Config.MaxMessageSize)
	s.Conn.SetReadDeadline(time.Now().Add(s.Melody.Config.PongWait))

	s.Conn.SetPongHandler(func(string) error {
		s.Conn.SetReadDeadline(time.Now().Add(s.Melody.Config.PongWait))
		s.Melody.pongHandler(s)
		return nil
	})

	if s.Melody.closeHandler != nil {
		s.Conn.SetCloseHandler(func(code int, text string) error {
			return s.Melody.closeHandler(s, code, text)
		})
	}

	for {
		t, message, err := s.Conn.ReadMessage()

		if err != nil {
			s.Melody.errorHandler(s, err)
			break
		}

		if t == websocket.TextMessage {
			s.Melody.messageHandler(s, message)
		}

		if t == websocket.BinaryMessage {
			s.Melody.messageHandlerBinary(s, message)
		}
	}
}

// Write writes message to session.
func (s *Session) Write(msg []byte) error {
	if s.closed() {
		return errWriteToClosedSession
	}

	s.writeMessage(&envelope{t: websocket.TextMessage, msg: msg})

	return nil
}

// WriteBinary writes a binary message to session.
func (s *Session) WriteBinary(msg []byte) error {
	if s.closed() {
		return errWriteToClosedSession
	}

	s.writeMessage(&envelope{t: websocket.BinaryMessage, msg: msg})

	return nil
}

// Close closes session.
func (s *Session) Close() error {
	if s.closed() {
		return errSessionAlreadyClosed
	}

	s.writeMessage(&envelope{t: websocket.CloseMessage, msg: []byte{}})

	return nil
}

// CloseWithMsg closes the session with the provided payload.
// Use the FormatCloseMessage function to format a proper close message payload.
func (s *Session) CloseWithMsg(msg []byte) error {
	if s.closed() {
		return errSessionAlreadyClosed
	}

	s.writeMessage(&envelope{t: websocket.CloseMessage, msg: msg})

	return nil
}

//CloseWithErr closes the session with the provided error.
func (s *Session) CloseWithErr(err error) error {
	if s.closed() {
		return errSessionAlreadyClosed
	}

	s.writeMessage(&envelope{t: websocket.CloseMessage, msg: []byte(err.Error())})

	return nil
}

// Set is used to store a new key/value pair exclusivelly for this session.
// It also lazy initializes s.keys if it was not used previously.
func (s *Session) Set(key string, value interface{}) {
	s.rwmutex.Lock()
	if s.keys == nil {
		s.keys = make(map[string]interface{})
	}
	s.keys[key] = value
	s.rwmutex.Unlock()
}

// Get returns the value for the given key, ie: (value, true).
// If the value does not exists it returns (nil, false)
func (s *Session) Get(key string) (value interface{}, exists bool) {
	s.rwmutex.RLock()
	value, exists = s.keys[key]
	s.rwmutex.RUnlock()
	return
}

// MustGet returns the value for the given key if it exists, otherwise it panics.
func (s *Session) MustGet(key string) interface{} {
	if value, exists := s.Get(key); exists {
		return value
	}

	panic("Key \"" + key + "\" does not exist")
}

// IsClosed returns the status of the connection.
func (s *Session) IsClosed() bool {
	return s.closed()
}
