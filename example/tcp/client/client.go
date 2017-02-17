package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"

	"github.com/ooclab/es"
	"github.com/ooclab/es/link"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Printf("Usage: %s Address Tunnel\n", os.Args[0])
		fmt.Printf("Example: %s 127.0.0.1:3000 f:127.0.0.1:10080:127.0.0.1:8080", os.Args[0])
		os.Exit(1)
	}

	logrus.SetLevel(logrus.DebugLevel)

	conn, err := net.Dial("tcp", os.Args[1])
	if err != nil {
		panic(err)
	}

	l := link.NewLink(nil)
	go func() {
		time.Sleep(1 * time.Second) // FIXME!
		// localHost := "127.0.0.1"
		// localPort := 10080
		// remoteHost := "127.0.0.1"
		// remotePort := 3000
		// reverse := false
		localHost, localPort, remoteHost, remotePort, reverse, err := parseTunnel(os.Args[2])
		if err != nil {
			panic(err)
		}
		l.OpenTunnel(localHost, localPort, remoteHost, remotePort, reverse)
	}()

	ec := es.NewBaseConn(conn)
	l.Bind(ec)
	l.Close()
}

func parseTunnel(value string) (localHost string, localPort int, remoteHost string, remotePort int, reverse bool, err error) {
	L := strings.Split(value, ":")
	if len(L) != 5 {
		fmt.Println("tunnel format: \"r|f:local_host:local_port:remote_host:remote_port\"")
		err = errors.New("tunnel map is wrong: " + value)
		return
	}

	localHost = L[1]
	remoteHost = L[3]
	switch L[0] {
	case "r", "R":
		reverse = true
	case "f", "F":
		reverse = false
	default:
		err = errors.New("wrong tunnel map")
		return
	}
	localPort, err = strconv.Atoi(L[2])
	if err != nil {
		return
	}
	remotePort, err = strconv.Atoi(L[4])
	if err != nil {
		return
	}

	return
}
