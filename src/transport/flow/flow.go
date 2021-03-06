// Package flow can be used to test MQTT packet flows.
package flow

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"packet"
)

// A Conn defines an abstract interface for connections used with a Flow.
type Conn interface {
	Send(pkt packet.GenericPacket) error
	Receive() (packet.GenericPacket, error)
	Close() error
}

// The Pipe pipes packets from Send to Receive.
type Pipe struct {
	pipe  chan packet.GenericPacket
	close chan struct{}
}

// NewPipe returns a new Pipe.
func NewPipe() *Pipe {
	return &Pipe{
		pipe:  make(chan packet.GenericPacket),
		close: make(chan struct{}),
	}
}

// Send returns packet on next Receive call.
func (conn *Pipe) Send(pkt packet.GenericPacket) error {
	select {
	case conn.pipe <- pkt:
		return nil
	case <-conn.close:
		return errors.New("already closed")
	}
}

// Receive returns the packet being sent with Send.
func (conn *Pipe) Receive() (packet.GenericPacket, error) {
	select {
	case pkt := <-conn.pipe:
		return pkt, nil
	case <-conn.close:
		return nil, io.EOF
	}
}

// Close will close the conn and let Send and Receive return errors.
func (conn *Pipe) Close() error {
	close(conn.close)
	return nil
}

// All available action types.
const (
	actionSend byte = iota
	actionReceive
	actionSkip
	actionWait
	actionRun
	actionDelay
	actionClose
	actionEnd
)

// An Action is a step in a flow.
type action struct {
	kind     byte
	packet   packet.GenericPacket
	fn       func()
	ch       chan struct{}
	duration time.Duration
}

// A Flow is a sequence of actions that can be tested against a connection.
type Flow struct {
	actions []*action
}

// New returns a new flow.
func New() *Flow {
	return &Flow{
		actions: make([]*action, 0),
	}
}

// Send will send and one packet.
func (f *Flow) Send(pkt packet.GenericPacket) *Flow {
	f.add(&action{
		kind:   actionSend,
		packet: pkt,
	})

	return f
}

// Receive will receive and match one packet.
func (f *Flow) Receive(pkt packet.GenericPacket) *Flow {
	f.add(&action{
		kind:   actionReceive,
		packet: pkt,
	})

	return f
}

// Skip will receive one packet without matching it.
func (f *Flow) Skip() *Flow {
	f.add(&action{
		kind: actionSkip,
	})

	return f
}

// Wait will wait until the specified channel is closed.
func (f *Flow) Wait(ch chan struct{}) *Flow {
	f.add(&action{
		kind: actionWait,
		ch:   ch,
	})

	return f
}

// Run will call the supplied function and wait until it returns.
func (f *Flow) Run(fn func()) *Flow {
	f.add(&action{
		kind: actionRun,
		fn:   fn,
	})

	return f
}

// Delay will suspend the flow using the specified duration.
func (f *Flow) Delay(d time.Duration) *Flow {
	f.add(&action{
		kind:     actionDelay,
		duration: d,
	})

	return f
}

// Close will immediately close the connection.
func (f *Flow) Close() *Flow {
	f.add(&action{
		kind: actionClose,
	})

	return f
}

// End will match proper connection close.
func (f *Flow) End() *Flow {
	f.add(&action{
		kind: actionEnd,
	})

	return f
}

// Test starts the flow on the given Conn and reports to the specified test.
func (f *Flow) Test(conn Conn) error {
	for _, action := range f.actions {
		switch action.kind {
		case actionSend:
			err := conn.Send(action.packet)
			if err != nil {
				return fmt.Errorf("error sending packet: %v", err)
			}
		case actionReceive:
			pkt, err := conn.Receive()
			if err != nil {
				return fmt.Errorf("expected to receive a packet but got error: %v", err)
			}

			if want, got := action.packet.String(), pkt.String(); want != got {
				return fmt.Errorf("expected packet of %q but got %q", want, got)
			}
		case actionSkip:
			_, err := conn.Receive()
			if err != nil {
				return fmt.Errorf("expected to skip over a received packet but got error: %v", err)
			}
		case actionWait:
			<-action.ch
		case actionRun:
			action.fn()
		case actionDelay:
			time.Sleep(action.duration)
		case actionClose:
			err := conn.Close()
			if err != nil {
				return fmt.Errorf("expected connection to close successfully but got error: %v", err)
			}
		case actionEnd:
			pkt, err := conn.Receive()
			if err != nil && !strings.Contains(err.Error(), "EOF") {
				return fmt.Errorf("expected EOF but got %v", err)
			}
			if pkt != nil {
				return fmt.Errorf("expected no packet but got %v", pkt)
			}
		}
	}

	return nil
}

// TestAsync starts the flow on the given Conn and reports to the specified test
// asynchronously.
func (f *Flow) TestAsync(conn Conn, timeout time.Duration) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		select {
		case <-time.After(timeout):
			errCh <- errors.New("timed out waiting for flow to complete")
		case errCh <- f.Test(conn):
		}
	}()

	return errCh
}

// add will add the specified action.
func (f *Flow) add(action *action) {
	f.actions = append(f.actions, action)
}
