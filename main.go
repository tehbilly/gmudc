package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/tehbilly/gmudc/telnet"
)

var (
	writeChan = make(chan []byte, 100)

	mudAddr = flag.String("addr", "imperian.com:23", "address [host:port] of the MUD server to connect to")
)

func main() {
	flag.Parse()

	conn := telnet.New()
	err := conn.Dial("tcp", *mudAddr)
	if err != nil {
		log.Println(err)
	}
	defer conn.Close()

	quit := make(chan bool)
	go readLoop(conn, quit)
	go writeLoop(conn, quit)

	for {
		select {
		case w := <-writeChan:
			conn.Write(w)
		case <-quit:
			break
		}
	}
}

func writeLoop(t *telnet.Connection, quit chan bool) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		if line[0] == '/' {
			//localCommands(t, line[1:])
		} else {
			sendLine(t, line)
		}
	}
	quit <- true
}

func readLoop(cr io.Reader, quit chan bool) {
	br := bufio.NewReader(cr)
	for {
		b := make([]byte, 1)
		_, err := br.Read(b)
		if err == io.EOF {
			quit <- true
			break
		}

		if err != nil {
			fmt.Println("Error in readLoop:", err)
			quit <- true
			break
		}

		os.Stdout.Write(b)
	}
}

func sendLine(t *telnet.Connection, line string) {
	line = strings.TrimSuffix(line, "\r\n")
	ob := make([]byte, len(line)+1)
	copy(ob, line)
	ob[len(line)] = '\n'
	_, err := t.Write(ob)
	if err != nil {
		fmt.Println("Conn write error:", err)
	}
}
