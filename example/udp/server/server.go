package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net"
	"os"

	"github.com/ooclab/es/proto/udp"
	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

func main() {
	addr, err := net.ResolveUDPAddr("udp", os.Args[1])
	if err != nil {
		fmt.Println("reslove udp addr failed: ", err)
		return
	}
	// addr := net.UDPAddr{
	// 	Port: 1234,
	// 	IP:   net.ParseIP("115.28.213.159"),
	// }
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Printf("Some error %v\n", err)
		return
	}
	sock, err := udp.NewServerSocket(conn)
	fmt.Println("sock, err = ", sock, err)
	defer conn.Close()

	for {
		conn, err := sock.Accept()
		if err != nil {
			fmt.Println("accept failed: ", err)
			break
		}
		fmt.Println("accept client: ", conn)
		go func() {
			fmt.Println("start client recv: ", conn)
			defer func() { fmt.Println("quit conn: ", conn) }()
			for {
				msg, err := conn.RecvMsg()
				if err != nil {
					fmt.Printf("recv msg failed: %s\n", err)
					return
				}
				rc := md5.Sum(msg)
				fmt.Printf("%9d <-- recv %s\n", len(msg), hex.EncodeToString(rc[:]))
				err = conn.SendMsg(msg)
				if err != nil {
					fmt.Println("send msg failed: ", err)
					return
				}
			}
		}()
	}

}
