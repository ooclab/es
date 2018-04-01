package main

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/ooclab/es/proto/udp"
	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s RemoteAddress\n", os.Args[0])
		return
	}
	// laddr, err := net.ResolveUDPAddr("udp", os.Args[1])
	// if err != nil {
	// 	logrus.Errorf("resolve local udp addr %s failed: %s", os.Args[1], err)
	// 	return
	// }

	raddr, err := net.ResolveUDPAddr("udp", os.Args[1])
	if err != nil {
		logrus.Errorf("resolve remote udp addr %s failed: %s", os.Args[2], err)
		return
	}
	conn, err := net.ListenUDP("udp", nil)
	// conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		fmt.Printf("Some error %v", err)
		return
	}

	sock, clientConn, err := udp.NewClientSocket(conn, raddr)
	if err != nil {
		fmt.Printf("create client socket failed: %s", err)
		return
	}
	fmt.Println("sock = ", sock)

	maxSize := 1024 * 1024 * 16
	var start time.Time
	var t time.Duration
	var speed float64
	for j := 0; j < 1; j++ {
		for i := 2; i <= maxSize; i = i * 2 {
			b := make([]byte, i+1)
			rand.Read(b)
			sc := md5.Sum(b)

			// Send
			start = time.Now()
			if err := clientConn.SendMsg(b); err != nil {
				logrus.Errorf("SendMsg failed: %s", err)
				break
			}
			t = time.Since(start)
			speed = (float64(i+1) / t.Seconds()) / (1024 * 1024)
			fmt.Printf("%9d --> send %s %16s %16f M/s\n", len(b), hex.EncodeToString(sc[:]), t, speed)

			msg, err := clientConn.RecvMsg()
			if err != nil {
				logrus.Errorf("RecvMsg failed: %s", err)
				break
			}

			// Recv
			// rc := md5.Sum(msg)
			// fmt.Printf("%9d <-- recv %s\n", len(msg), hex.EncodeToString(rc[:]))
			if md5.Sum(msg) != sc {
				logrus.Errorf("msg is mismatch")
				break
			}
		}
	}

	clientConn.Close()
}
