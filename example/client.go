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

	conn, err := net.Dial("tcp", os.Args[1])
	if err != nil {
		panic(err)
	}

	l := link.NewLink(nil)
	l.Bind(conn)

	localHost := "127.0.0.1"
	localPort := 10080
	remoteHost := "127.0.0.1"
	remotePort := 3000
	reverse := false
	l.OpenTunnel(localHost, localPort, remoteHost, remotePort, reverse)

	l.WaitDisconnected()
}
