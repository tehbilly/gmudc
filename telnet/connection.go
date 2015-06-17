package telnet

import (
	"bytes"
	"net"
	"sync"
)

// Connection represents a connection and supporting actors. This object keeps
// track of all handlers and performs the necessary stream interception required
// to perform fancy things like triggers and aliases.
type Connection struct {
	conn      net.Conn
	processor *tnProcessor
	// Event/msg (out-of-band) handlers
	handlers     []*handlerRunner
	handlerMutex sync.Mutex
	// Some channels for command sequences?
}

// AddHandler adds a new out-of-band msg handler that will be invoked for
// IAC + ... commands. It is the responsibility of the handler to check for
// how applicable the message is for them. bytes.HasPrefix(msg, []byte{...})
// works well for determining this.
//
// Each handler will run in its own goroutine, and has a small buffer of pending
// messages it can handle. It is the responsibility of the handler to return as
// quickly as possible to prevent itself from being forcibly removed.
func (t *Connection) AddHandler(h Handler) {
	runner := &handlerRunner{
		msgChan: make(chan []byte, 100),
		handler: h,
	}
	t.handlerMutex.Lock()
	defer t.handlerMutex.Unlock()
	go runner.run()
	t.handlers = append(t.handlers, runner)
}

// Dial will attempt to make a connection to the type/address specific
// eg: conn.Dial("tcp", "somewhere.com:23")
func (t *Connection) Dial(network string, url string) error {
	c, err := net.Dial(network, url)
	if err != nil {
		return err
	}
	t.conn = c

	go startSystemHandlers(t)
	return nil
}

// New creates a new instance with appropriately initialized private fields.
// Create a Connection without using New() at your own risk!
func New() *Connection {
	tc := &Connection{
		processor: newTelnetProcessor(),
		handlers:  make([]*handlerRunner, 0),
	}
	tc.processor.conn = tc
	return tc
}

// Connection implements the io.Writer interface.
func (t *Connection) Write(b []byte) (int, error) {
	return t.conn.Write(b)
}

// Connection implements the io.Reader interface.
func (t *Connection) Read(b []byte) (int, error) {
	cb := make([]byte, 1024)
	n, err := t.conn.Read(cb)
	t.processor.processBytes(cb[:n])
	if err != nil {
		return n, err
	}

	return t.processor.Read(b)
}

// Close closes the underlying net.Conn
func (t *Connection) Close() error {
	return t.conn.Close()
}

// SendCommand formats and sends a command (series of tnSeq) to the server.
// eg: conn.SendCommand(telnet.WILL, telnet.GMCP).
// IAC is prefixed already, so there's no need to prepend it.
func (t *Connection) SendCommand(command ...tnSeq) {
	t.conn.Write(buildCommand(command...))
}

// Internal function to IACize some commands and turn 'em into bytes
func buildCommand(c ...tnSeq) []byte {
	// cmd := make([]byte, len(c)+1)
	var cmd bytes.Buffer

	cmd.WriteByte(byte(IAC))

	// Don't want to double up on IAC
	if c[0] == IAC {
		c = c[1:]
	}

	for _, v := range c {
		cmd.WriteByte(byte(v))
	}

	return cmd.Bytes()
}
