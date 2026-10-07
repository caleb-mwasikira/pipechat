// Package client is the Pipechat terminal client: the server's API, the
// WebSocket that delivers incoming messages, local storage, invitation email,
// file transfer and the terminal screen.
package client

import (
	"context"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// Events the connection reports to the screen.
type ServerUp struct{}              // the server answers and nobody is logged in: time to log in
type ServerDown struct{ Err error } // the server cannot be reached
type SocketUp struct{}              // the WebSocket is open: incoming messages arrive
type SocketDown struct{ Err error } // the WebSocket ended; the session is gone
type SocketEvent struct{ Event Event }

// Connect keeps the connection to the server alive in the background. Everything
// that happens (server up or down, incoming messages, finished requests) is
// handed to report, which is the screen's Send function.
//
// The background loop does one of two things:
//
//   - nobody is logged in: check that the server answers, and tell the screen
//     (ServerUp) so it can log in;
//   - someone is logged in: hold a WebSocket open to receive incoming messages.
//     When it ends, the session is forgotten and the loop starts over.
func (s *Server) Connect(report func(tea.Msg)) {
	s.report = report
	go s.keepConnected()
}

func (s *Server) keepConnected() {
	announced := false
	for !s.stopped.Load() {
		token := s.Token()
		if token == "" {
			err := s.checkServer()
			switch {
			case err != nil:
				s.emit(ServerDown{Err: err})
				announced = false
			case !announced:
				s.emit(ServerUp{})
				announced = true
			}
			select {
			case <-s.sessionSet:
			case <-s.done:
			case <-time.After(3 * time.Second):
			}
			continue
		}

		err := s.runSocket(token)
		s.endSession(token)
		s.emit(SocketDown{Err: err})
		announced = false
		select {
		case <-s.done:
		case <-time.After(time.Second):
		}
	}
}

func (s *Server) checkServer() error {
	client := http.Client{Timeout: 5 * time.Second}
	response, err := client.Get(s.url + "/up")
	if err != nil {
		return err
	}
	response.Body.Close()
	return nil
}

func (s *Server) webSocketURL() string {
	address := s.url
	if strings.HasPrefix(address, "https://") {
		address = "wss://" + strings.TrimPrefix(address, "https://")
	} else {
		address = "ws://" + strings.TrimPrefix(address, "http://")
	}
	return address + "/api/ws"
}

// runSocket holds the WebSocket open and reports every event until it ends.
func (s *Server) runSocket(token string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.mutex.Lock()
	s.stopSocket = cancel
	s.mutex.Unlock()

	dialContext, stopDialing := context.WithTimeout(ctx, 10*time.Second)
	connection, _, err := websocket.Dial(dialContext, s.webSocketURL(), &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer " + token}},
	})
	stopDialing()
	if err != nil {
		return err
	}
	defer connection.CloseNow()
	connection.SetReadLimit(1 << 20)
	s.emit(SocketUp{})

	// Notice a server that vanished without closing the connection.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pingContext, stop := context.WithTimeout(ctx, 10*time.Second)
				err := connection.Ping(pingContext)
				stop()
				if err != nil {
					cancel()
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		var event Event
		if err := wsjson.Read(ctx, connection, &event); err != nil {
			return err
		}
		s.emit(SocketEvent{Event: event})
	}
}
