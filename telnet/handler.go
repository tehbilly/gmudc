package telnet

import (
	"bytes"
	"errors"
	"fmt"
)

var (
	deadHandler = errors.New("Handler unresponsive.")
)

// Handler will handle an out-of-band (IAC + ...) command event
type Handler interface {
	Handle([]byte)
}

type handlerRunner struct {
	handler Handler
	msgChan chan []byte
	closed  bool
}

func (r *handlerRunner) run() {
	for {
		msg, ok := <-r.msgChan
		if !ok {
			break
		}

		r.handler.Handle(msg)
	}
}

func (r *handlerRunner) send(msg []byte) error {
	if r.closed {
		return nil
	}

	select {
	case r.msgChan <- msg:
		return nil

	default:
		close(r.msgChan)
		return deadHandler
	}
}

func addSystemHandler(c *Connection, h Handler) {
	runner := &handlerRunner{
		msgChan: make(chan []byte, 1024),
		handler: h,
	}
	c.handlerMutex.Lock()
	defer c.handlerMutex.Unlock()
	go runner.run()
	c.handlers = append(c.handlers, runner)
}

// Some basic system handlers
func startSystemHandlers(c *Connection) {
	// Definitely need a separate processing stream for inbound GMCP messages
	addSystemHandler(c, &gmcpInboundHandler{})
	// A very simple handler that tells the server if we want to accept GMCP or not
	addSystemHandler(c, &politeClientHandler{conn: c})
}

// Do you want gmcp? Yes please
type politeClientHandler struct {
	saidHello bool
	conn      *Connection
}

func (h *politeClientHandler) sendGMCP(message string) {
	var gmsg bytes.Buffer
	gmsg.Write([]byte{byte(IAC), byte(SB), byte(GMCP)})
	gmsg.WriteString(message)
	gmsg.Write([]byte{byte(IAC), byte(SE)})
	h.conn.Write(gmsg.Bytes())
}

func (h *politeClientHandler) Handle(msg []byte) {
	// This is a very good place to do our on-connect stuff.
	// TODO: This should probably be changed to a proper connection-detecting thingie
	if hasSeqPrefix(msg, IAC, WILL, GMCP) {
		fmt.Printf("IAC WILL GMCP. Time to reply\n")
		// Yes, yes we will GMCP
		h.conn.SendCommand(DO, GMCP)
		h.sendGMCP(`Core.Hello {"client": "gmudc", "version": "0.0.2"}`)
		h.sendGMCP(`Core.Supports.Set [ "Char 1", "Char.Skills 1", "Char.Items 1", "Comm.Channel 1", "Room 1", "IRE.Rift 1"]`)
	}
}

type GMCPHandler interface {
	HandleGMCP(string, []byte)
}

type gmcpInboundHandler struct {
}

func (h *gmcpInboundHandler) Handle(msg []byte) {
	msgType := bToSeq(msg[0])
	msg = msg[1:]
	switch msgType {
	case GMCP:
		var module, data []byte
		si := bytes.IndexByte(msg, ' ')
		if si == -1 {
			module = msg
		} else {
			module = msg[:si]
			data = msg[si+1:]
		}
		// So here we are. We have a module name, and all data afterwards.
		fmt.Printf("GMCP %s [len(data)==%d]\n", module, len(data))
		// I guess here we can have specialized handlers.
		// HandleGMCP(module, data string) perhaps ?
		//
		// Or maybe a struct?
		// type GMCPMessage struct {
		//   Module string
		//   Data   []byte // For to easily decode/unmarshal!
		// }
		//
		// Either way, I'd like to have the filtering be handled within the library,
		// and not force the users to filter.
	}
}
