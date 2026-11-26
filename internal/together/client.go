package together

import (
	"sync"
	"sync/atomic"
)

var clientSeq atomic.Uint64

// Client is one WebSocket connection to a lobby. The lobby queues messages into its bounded
// send buffer without ever blocking: a client whose buffer is full is closed (slow consumer)
// and resyncs when it reconnects. The connection's writer drains Send until Done is closed.
type Client struct {
	id       uint64
	userID   uint
	username string
	lobby    *lobby

	send      chan []byte
	done      chan struct{}
	closeOnce sync.Once
	// set once, before done is closed
	closeCode   int
	closeReason string

	// guarded by the lobby lock
	buffering  bool
	reportSeq  uint64
	reportItem uint
	// sync point the client joined in: it does not hold the lobby for that one
	joinedSync uint64
	moderate   bool
}

func newClient(userID uint, username string, buffer int) *Client {
	if buffer < 1 {
		buffer = 1
	}
	return &Client{
		id:       clientSeq.Add(1),
		userID:   userID,
		username: username,
		send:     make(chan []byte, buffer),
		done:     make(chan struct{}),
	}
}

// UserID is the connected user.
func (c *Client) UserID() uint { return c.userID }

// Send gives the messages to write to the connection.
func (c *Client) Send() <-chan []byte { return c.send }

// Done is closed when the server closes the client (kick, slow consumer, lobby closed...).
func (c *Client) Done() <-chan struct{} { return c.done }

// CloseStatus is the WebSocket close code and reason, once Done is closed.
func (c *Client) CloseStatus() (int, string) {
	return c.closeCode, c.closeReason
}

// enqueue queues a message without blocking; a full buffer closes the client.
func (c *Client) enqueue(b []byte) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case c.send <- b:
		return true
	default:
		c.close(ClosePolicy, "send buffer full")
		return false
	}
}

// close ends the client with a WebSocket close code (only the first call counts).
func (c *Client) close(code int, reason string) {
	c.closeOnce.Do(func() {
		c.closeCode = code
		c.closeReason = reason
		close(c.done)
	})
}
