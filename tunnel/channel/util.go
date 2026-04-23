package channel

import (
	"net"

	logrus "github.com/ooclab/es/logger"
)

func closeConn(conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			logrus.Warn("closeConn recovered: ", r)
		}
	}()
	conn.Close()
}
