package test

import (
	"crypto/rand"
	"fmt"
	"net"
	"testing"

	"github.com/Sirupsen/logrus"

	"github.com/ooclab/es/isession"
	"github.com/ooclab/es/link"
)

// For test
// func init() {
// 	logrus.SetFormatter(&logrus.TextFormatter{
// 		FullTimestamp:   true,
// 		TimestampFormat: "01/02 15:04:05",
// 	})
// 	logrus.SetLevel(logrus.DebugLevel)
// }

func startServer() (port int, err error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Println("listen error:", err)
		return 0, err
	}

	port = l.Addr().(*net.TCPAddr).Port
	// fmt.Printf("start listen on %d\n", port)

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				fmt.Println("accept new conn error: ", err)
				continue // TODO: fix me!
			}

			go func() {
				l := link.NewLink(nil)
				l.Bind(conn)
				l.WaitDisconnected()
			}()
		}
	}()

	return port, nil
}

func tcp_connect(addr_s string) (conn net.Conn, err error) {
	// fmt.Printf("try connect to relay server: %s\n", addr_s)
	addr, err := net.ResolveTCPAddr("tcp", addr_s)
	if err != nil {
		fmt.Println("resolve relay-server (%s) failed: %s", addr, err)
		return
	}
	conn, err = net.DialTCP("tcp", nil, addr)
	if err != nil {
		fmt.Println("dial %s failed: %s", addr, err.Error())
		return
	}
	// fmt.Printf("connect to relay server %s success\n", conn.RemoteAddr())

	return
}

func connectServer(addr string) *link.Link {
	conn, err := tcp_connect(addr)
	if err != nil {
		panic(err)
	}
	l := link.NewLink(nil)
	l.Bind(conn)
	// FIXME: quit it not a good choice for testcase!
	go func() {
		l.WaitDisconnected()
		l.Close()
	}()
	return l
}

func testEq(a, b []byte) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func Benchmark_LinkInnerSessionSingle(b *testing.B) {
	port, _ := startServer()
	l := connectServer(fmt.Sprintf("127.0.0.1:%d", port))

	s, _ := l.OpenInnerSession()
	for i := 0; i < b.N; i++ { //use b.N for looping
		s.Post("/echo", []byte("Ping"))
	}
}

func Benchmark_LinkInnerSessionMulti(b *testing.B) {
	port, _ := startServer()
	l := connectServer(fmt.Sprintf("127.0.0.1:%d", port))

	for i := 0; i < b.N; i++ { //use b.N for looping
		s, _ := l.OpenInnerSession()
		s.Post("/echo", []byte("Ping"))
	}
}

func Test_LinkInnerSessionMinimalFrame(t *testing.T) {
	port, _ := startServer()
	l := connectServer(fmt.Sprintf("127.0.0.1:%d", port))

	s, _ := l.OpenInnerSession()
	for i := 0; i < 1025; i++ {
		if !testLinkInnerSession(s, i) {
			t.Error("response and request mismatch!")
			return
		}
	}
}

// FIXME! remote endpoint would close connection when send large message, test it someday!
// func Test_LinkInnerSessionLargeFrame(t *testing.T) {
// 	port, _ := startServer()
// 	l := connectServer(fmt.Sprintf("127.0.0.1:%d", port))
// 	defer l.Close()
//
// 	s, _ := l.OpenInnerSession()
// 	// FIXME: session request package is large than raw payload
// 	for _, i := range []int{4194303, 4194304, 4194305} {
// 		if !testLinkInnerSession(s, i) {
// 			t.Error("response and request mismatch!")
// 			return
// 		}
// 	}
// }

func testLinkInnerSession(s *isession.Session, length int) bool {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		logrus.Errorf("read rand failed: %s", err)
		return false
	}

	resp, err := s.Post("/echo", b)
	if err != nil {
		logrus.Error("request failed:", resp, err, length)
		return false
	}
	return testEq(b, resp.Body)
}
