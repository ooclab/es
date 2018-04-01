package main

import (
	"fmt"
	"net"
	"os"

	"github.com/ooclab/es"
	"github.com/ooclab/es/link"
	"github.com/sirupsen/logrus"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s Address\n", os.Args[0])
		os.Exit(1)
	}

	logrus.SetLevel(logrus.DebugLevel)

	l, err := net.Listen("tcp", os.Args[1])
	if err != nil {
		panic(err)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			panic(err)
		}
		logrus.Infof("accept client %s", conn.RemoteAddr())

		go handleClient(conn)
	}
}

func handleClient(_conn net.Conn) {
	l := link.NewLink(nil)
	conn := es.NewBaseConn(_conn)
	l.Bind(conn)
	l.Close()
}
