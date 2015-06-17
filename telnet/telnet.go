package telnet

//go:generate stringer -type tnSeq -output telnet_string.go

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
)

type tnSeq uint8

func hasSeqPrefix(check []byte, seq ...tnSeq) bool {
	bz := make([]byte, 0)
	for _, s := range seq {
		bz = append(bz, byte(s))
	}

	return bytes.HasPrefix(check, bz)
}

func bytesToSeq(b []byte) []tnSeq {
	is := make([]tnSeq, 0)
	bb := bytes.NewReader(b)
	for i := 0; i < len(b); i++ {
		var ib tnSeq
		err := binary.Read(bb, binary.BigEndian, &ib)
		if err != nil {
			fmt.Println("Oh no! An error!", err)
		}
		is = append(is, ib)
	}
	return is
}

func bToSeq(b byte) tnSeq {
	var is tnSeq
	bb := bytes.NewReader([]byte{b})
	binary.Read(bb, binary.BigEndian, &is)
	return is
}

const (
	NUL  tnSeq = 0x00 // NULL, noop
	ECHO tnSeq = 0x01 // Echo
	SGA  tnSeq = 0x03 // Suppress 'Go Ahead'
	ST   tnSeq = 0x05 // Status
	TM   tnSeq = 0x06 // Timing mark
	BEL  tnSeq = 0x07 // Bell
	BS   tnSeq = 0x08 // Backspace
	HT   tnSeq = 0x09 // Horizontal tab
	LF   tnSeq = 0x0A // Line feed
	FF   tnSeq = 0x0C // Form feed
	CR   tnSeq = 0x0D // Carriage return
	TT   tnSeq = 0x18 // Terminal type
	EOR  tnSeq = 0x19 // End of record
	WS   tnSeq = 0x1F // Window size
	TS   tnSeq = 0x20 // Terminal speed
	RFC  tnSeq = 0x21 // Remote flow control
	LM   tnSeq = 0x22 // Line mode
	EV   tnSeq = 0x24 // Environment variables
	SE   tnSeq = 0xF0 // End of subnegotiation
	NOP  tnSeq = 0xF1 // No operation
	DM   tnSeq = 0xF2 // Data mark. The data stream portion of a Synch. This should always be accompanied by a TCP Urgent notification.
	BRK  tnSeq = 0xF3 // Break. NVT char BRK.
	IP   tnSeq = 0xF4 // Interrupt process
	AO   tnSeq = 0xF5 // Abort output
	AYT  tnSeq = 0xF6 // Are You There
	EC   tnSeq = 0xF7 // Erase character
	EL   tnSeq = 0xF8 // Erase line
	GA   tnSeq = 0xF9 // Go ahead
	SB   tnSeq = 0xFA // Start of subnegotiation
	WILL tnSeq = 0xFB
	WONT tnSeq = 0xFC
	DO   tnSeq = 0xFD
	DONT tnSeq = 0xFE
	IAC  tnSeq = 0xFF // Interpret As Command
	CMP1 tnSeq = 0x55 // MCCP Compress
	CMP2 tnSeq = 0x56 // MCCP Compress2
	ATCP tnSeq = 0xC8 // Achaea Telnet Client Protocol
	GMCP tnSeq = 0xC9 // Generic MUD Communication Protocol
)

var (
	seqChan  = make(chan []tnSeq, 100)
	readChan = make(chan []byte, 100)
)

// Finite state machine!
type tnState int

// States for the machine
const (
	inDefault tnState = iota
	inIAC
	inSB
	inCapSB
	inEscIAC
)

// Intercept reads, process telnet escapes, etc.
type tnProcessor struct {
	conn  *Connection
	state tnState

	currentSub  byte
	subData     map[byte][]byte
	cappedBytes []byte
	cleanBytes  []byte
}

func newTelnetProcessor() *tnProcessor {
	processor := &tnProcessor{
		state:      inDefault,
		currentSub: byte(NUL),
		subData:    make(map[byte][]byte),
	}
	return processor
}

// We're an io.Reader
func (p *tnProcessor) Read(b []byte) (int, error) {
	max := len(b)
	n := 0

	if max >= len(p.cleanBytes) {
		n = len(p.cleanBytes)
	} else {
		n = max
	}

	for i := 0; i < n; i++ {
		b[i] = p.cleanBytes[i]
	}

	p.cleanBytes = p.cleanBytes[n:]

	return n, nil
}

func (p *tnProcessor) capture(b byte) {
	p.cappedBytes = append(p.cappedBytes, b)
}

func (p *tnProcessor) dontCap(b byte) {
	p.cleanBytes = append(p.cleanBytes, b)
}

func (p *tnProcessor) clearSubData(d byte) {
	p.subData[d] = []byte{}
}

func (p *tnProcessor) capSubData(d byte, b byte) {
	p.subData[d] = append(p.subData[d], b)
}

func (p *tnProcessor) doHandlers(bs []byte) {
	for i, h := range p.conn.handlers {
		// A new copy of the slice. Gotta be safe.
		m := make([]byte, len(bs))
		copy(m, bs)
		err := h.send(m)
		if err != nil {
			go func() { // Using a mutex, so safe to launch these off
				p.conn.handlerMutex.Lock()
				defer p.conn.handlerMutex.Unlock()
				p.conn.handlers = append(p.conn.handlers[:i], p.conn.handlers[i+1:]...)
			}()
		}
	}
}

func (p *tnProcessor) processBytes(bytes []byte) {
	for _, b := range bytes {
		p.processByte(b)
	}
}

func (p *tnProcessor) processByte(b byte) {
	// Byte as sequence
	bs := tnSeq(b)
	switch p.state {
	case inDefault:
		if bs == IAC {
			p.state = inIAC
			p.capture(b)
		} else {
			p.dontCap(b)
		}

	case inIAC:
		if bs == WILL || bs == WONT || bs == DO || bs == DONT {
			// Stay in this state
		} else if b == byte(SB) {
			p.state = inSB
		} else {
			p.state = inDefault
		}
		p.capture(b)

		if p.state == inDefault { // This means a command is over!
			p.doHandlers(p.cappedBytes)
			p.cappedBytes = []byte{}
		}

	case inSB:
		p.capture(b)
		p.currentSub = b
		p.state = inCapSB
		p.clearSubData(b)

	case inCapSB:
		if bs == IAC {
			p.state = inEscIAC
		} else {
			p.capSubData(p.currentSub, b)
		}

	case inEscIAC:
		if bs == IAC {
			p.state = inCapSB
			p.capSubData(p.currentSub, b)
		} else {
			p.subDataFinished(p.currentSub)
			//p.currentSub = byte(NUL)
			p.state = inDefault
			p.processByte(byte(IAC))
			p.processByte(b)
		}
	}
}

func seqToString(bytes []byte) string {
	seqString := []string{}
	for _, s := range bytes {
		seqString = append(seqString, string(tnSeq(s)))
	}
	return strings.Join(seqString, " ")
}

func (p *tnProcessor) subDataFinished(d byte) {
	p.doHandlers(append([]byte{d}, p.subData[d]...))
	// Probably where we should handle subdata type handlers. GMCP, etc.
}
