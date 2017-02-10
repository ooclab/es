package main

import (
	"fmt"
	"net"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/ooclab/es/link"
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

func handleClient(conn net.Conn) {
	l := link.NewLink(nil)
	errCh := l.Join(conn)
	<-errCh
}
