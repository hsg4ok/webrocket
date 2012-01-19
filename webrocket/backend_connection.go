package webrocket

import (
	"bufio"
	"net"
	"sync"
)

// backendConnection implements a wrapper for the TCP connection providing
// some concurrency tricks.
type backendConnection struct {
	// The underlaying connection.
	conn net.Conn
	// The parent endpoint.
	endpoint *BackendEndpoint
	// Internal semaphore.
	mtx sync.Mutex
}

// Internal constructor
// -----------------------------------------------------------------------------

// newBackendConnection wrapps given connection into new backend connection
// object.
//
// endpoint - The parent endpoint.
// conn     - The connection to be wrapped.
//
// Returns new backend connection.
func newBackendConnection(endpoint *BackendEndpoint, conn net.Conn) *backendConnection {
	return &backendConnection{
		conn:     conn,
		endpoint: endpoint,
	}
}

// Exported
// -----------------------------------------------------------------------------

// Recv receives the data from the underlaying connection and maps it to
// the backend request structure. If there's no data to read it shall block
// until new data will appear.
//
// Returns read request or an error if something went wrong.
func (c *backendConnection) Recv() (req *backendRequest, err error) {
	var msg = [][]byte{}
	var buf = bufio.NewReader(c.conn)
	var possibleEom = false
	for {
		chunk, err := buf.ReadSlice('\n')
		if err != nil {
			return nil, err
		}
		if string(chunk) == "\r\n" {
			// Seems like it's end of the message...
			if possibleEom {
				// .. yeap, it is!
				break
			}
			possibleEom = true
			continue
		} else {
			possibleEom = false
		}
		msg = append(msg[:], chunk[:len(chunk)-1])
	}
	// <<<
	// identity\n
	// \n
	// commany\n
	// ...
	// >>>
	if len(msg) < 3 {
		return
	}
	// Compose the request object.
	aid, cmd := msg[0], msg[2]
	req = newBackendRequest(c, aid, cmd, msg[3:])
	return
}

// Send packs the command and frames together and sends it to the client.
//
// cmd    - The command to be send.
// frames - The frames to be send.
//
// Returns an error if something went wrong.
func (c *backendConnection) Send(cmd string, frames ...string) (err error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	payload := cmd + "\n"
	for _, frame := range frames {
		payload += frame + "\n"
	}
	payload += "\r\n\r\n"
	_, err = c.conn.Write([]byte(payload))
	return
}

// SetTimeout sets a receiver's timeout of the underlaying connection. 
func (c *backendConnection) SetTimeout(nsec int64) {
	c.conn.SetReadTimeout(nsec)
}

// IsAlive returns whether the underlaying connection is alive or not.
func (c *backendConnection) IsAlive() bool {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.conn != nil
}

// Kill closes the underlaying connection.
func (c *backendConnection) Kill() {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}