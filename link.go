package es

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/Sirupsen/logrus"
)

const (
	LINK_FRAME_VERSION_DEFAULT uint8  = 2
	LINK_FRAME_FALG_DEFAULT    uint16 = 0
)

// frame type
const (
	LINK_FRAME_TYPE_INNERSESSION_REQ = 1
	LINK_FRAME_TYPE_INNERSESSION_REP = 2

	LINK_FRAME_TYPE_SESSION_OPEN   uint8 = 11
	LINK_FRAME_TYPE_SESSION_STREAM uint8 = 12
	LINK_FRAME_TYPE_SESSION_CLOSE  uint8 = 13

	LINK_FRAME_TYPE_TUNNEL_OPEN   uint8 = 21
	LINK_FRAME_TYPE_TUNNEL_STREAM uint8 = 22
	LINK_FRAME_TYPE_TUNNEL_CLOSE  uint8 = 23
)

var linkFrameHumanTypeMap = map[uint8]string{

	LINK_FRAME_TYPE_INNERSESSION_REQ: "INNERSESSION-REQ",
	LINK_FRAME_TYPE_INNERSESSION_REP: "INNERSESSION-REP",

	LINK_FRAME_TYPE_SESSION_OPEN:   "SESSION_OPEN",
	LINK_FRAME_TYPE_SESSION_STREAM: "SESSION_STREAM",
	LINK_FRAME_TYPE_SESSION_CLOSE:  "SESSION_CLOSE",

	LINK_FRAME_TYPE_TUNNEL_OPEN:   "TUNNEL_OPEN",
	LINK_FRAME_TYPE_TUNNEL_STREAM: "TUNNEL_STREAM",
	LINK_FRAME_TYPE_TUNNEL_CLOSE:  "TUNNEL_CLOSE",
}

const (
	LINK_FRAME_HEADER_LENGTH = 12
	LINK_FRAME_LENGTH_MAX    = uint32(16777216) // 16M
)

type LinkConfig struct {
}

type linkFrame struct {
	Version uint8
	Type    uint8
	Flag    uint16 // use 2**16 = 65536
	ID      uint32
	Length  uint32
	Payload []byte
}

func (frame *linkFrame) String() string {
	return fmt.Sprintf("frame V:%d T:%s F:%d ID:%d L:%d",
		frame.Version, getLinkFrameHumanType(frame.Type), frame.Flag, frame.ID, frame.Length)
}

func getLinkFrameHumanType(_type uint8) string {
	if v, ok := linkFrameHumanTypeMap[_type]; ok {
		return v
	}

	return fmt.Sprintf("UNKNOWN(%d)", _type)
}

// func (frame *linkFrame) DumpHeader() []byte {
// 	data := make([]byte, LINK_FRAME_HEADER_LENGTH)
// 	data[0] = frame.Version
// 	data[1] = frame.Type
// 	binary.BigEndian.PutUint16(data[2:4], frame.Flag)
// 	binary.BigEndian.PutUint32(data[4:8], frame.ID)
// 	binary.BigEndian.PutUint32(data[8:12], frame.Length)
// 	return data
// }

type Link struct {
	ID string // uuid ?

	// tunnel
	curTunnelID     uint32
	tunnelIDMutex   *sync.Mutex
	tunnelPool      map[uint32]*Tunnel
	tunnelPoolMutex *sync.Mutex

	innerSessionPool *InnerSessionPool

	conn             io.ReadWriteCloser
	connDisconnected chan bool
}

func NewLink(config *LinkConfig) *Link {
	return &Link{
		tunnelPool:       map[uint32]*Tunnel{},
		tunnelPoolMutex:  &sync.Mutex{},
		curTunnelID:      1,
		tunnelIDMutex:    &sync.Mutex{},
		innerSessionPool: newInnerSessionPool(),

		connDisconnected: make(chan bool, 1),
	}
}

func (l *Link) readFrame() (*linkFrame, error) {
	hdrData, err := mustRead(l.conn, LINK_FRAME_HEADER_LENGTH)
	if err != nil {
		return nil, err
	}

	frame := &linkFrame{
		Version: hdrData[0],
		Type:    hdrData[1],
		Flag:    binary.LittleEndian.Uint16(hdrData[2:4]),
		ID:      binary.LittleEndian.Uint32(hdrData[4:8]),
		Length:  binary.LittleEndian.Uint32(hdrData[8:12]),
	}

	if frame.Length > LINK_FRAME_LENGTH_MAX {
		logrus.Debugf("read link frame: %s", frame)
		return nil, ErrMessageLengthTooLarge
	}

	if frame.Length > 0 {
		frame.Payload, err = mustRead(l.conn, frame.Length)
		if err != nil {
			return nil, err
		}
	}

	return frame, nil
}

func (l *Link) writeFrame(_type uint8, id uint32, payload []byte) error {
	length := uint32(len(payload))
	if length > LINK_FRAME_LENGTH_MAX {
		return ErrMessageLengthTooLarge
	}
	hdrData := make([]byte, LINK_FRAME_HEADER_LENGTH)
	hdrData[0] = LINK_FRAME_VERSION_DEFAULT
	hdrData[1] = _type
	binary.LittleEndian.PutUint16(hdrData[2:4], LINK_FRAME_FALG_DEFAULT)
	binary.LittleEndian.PutUint32(hdrData[4:8], id)
	binary.LittleEndian.PutUint32(hdrData[8:12], length)

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, hdrData)
	if length > 0 {
		binary.Write(buf, binary.LittleEndian, payload)
	}

	// TODO: lock
	_, err := l.conn.Write(buf.Bytes())
	return err
}

func (l *Link) Bind(conn io.ReadWriteCloser) error {
	l.conn = conn

	go func() {
		// TODO:
		defer conn.Close()
		defer close(l.connDisconnected)

		// get "in message" and forward
		var frame *linkFrame
		var err error
		for {
			// header
			frame, err = l.readFrame()
			if err != nil {
				if err == io.EOF {
					logrus.Debug("conn is closed")
					break
				}

				// TODO
				logrus.Errorf("read link frame error: %s", err)
				break
			}

			// dispatch
			switch frame.Type {

			case LINK_FRAME_TYPE_INNERSESSION_REQ:
				err = l.handleInnerSessionRequest(frame)

			case LINK_FRAME_TYPE_INNERSESSION_REP:
				err = l.handleInnerSessionResponse(frame)

			case LINK_FRAME_TYPE_SESSION_STREAM:
				err = l.sessionStream(frame)

			case LINK_FRAME_TYPE_TUNNEL_OPEN:
				err = l.handleTunnelOpen(frame)
			case LINK_FRAME_TYPE_TUNNEL_STREAM:
				err = l.handleTunnelStream(frame)
			case LINK_FRAME_TYPE_TUNNEL_CLOSE:
				err = l.handleTunnelClose(frame)

			default:
				logrus.Errorf("unknown message: %s", frame)
				err = ErrMessageTypeUnknown

			}

			if err != nil {
				logrus.Errorf("handle message %s error: %s", frame, err)
				break
			}
		}
		logrus.Debug("HandleIn is stoped")
	}()

	return nil
}

func (l *Link) Close() error {
	return nil
}

func (l *Link) WaitDisconnected() error {
	<-l.connDisconnected
	logrus.Debugf("link %s is disconnected", l.ID)
	return nil
}

func (l *Link) OpenInnerSession() (*InnerSession, error) {
	return l.innerSessionPool.New(l)
}

func (l *Link) handleInnerSessionRequest(frame *linkFrame) error {
	// logrus.Debugf("inner session : handle in : %s", string(frame.Payload))
	// test: echo
	// fmt.Println("got session request frame:", frame)
	return l.writeFrame(LINK_FRAME_TYPE_INNERSESSION_REP, frame.ID, frame.Payload)
}

func (l *Link) handleInnerSessionResponse(frame *linkFrame) error {
	session := l.innerSessionPool.Get(frame.ID)
	if session == nil {
		return errors.New("no such inner session")
	}

	return session.HandleResponse(frame.Payload)
}

func (l *Link) sessionStream(frame *linkFrame) error {
	logrus.Infof("got session message: %s", frame)
	return nil
}

// OpenTunnel open a tunnel
func (l *Link) OpenTunnel(localHost string, localPort int, remoteHost string, remotePort int, reverse bool) (*Tunnel, error) {
	// send open tunnel message to remote endpoint
	payload, err := json.Marshal(TunnelConfig{
		LocalHost:  localHost,
		LocalPort:  localPort,
		RemoteHost: remoteHost,
		RemotePort: remotePort,
		Reverse:    reverse,
	})
	fmt.Println(payload, err)
	// go l.writeFrame(LINK_FRAME_TYPE_SESSION_OPEN, 0, payload)

	// success: open tunnel at local endpoint
	return nil, nil
}

func (l *Link) handleTunnelOpen(frame *linkFrame) error {
	logrus.Debugf("handle tunnel open: %s", frame)
	tc := TunnelConfig{}
	if err := json.Unmarshal(frame.Payload, &tc); err != nil {
		return err
	}

	logrus.Debugf("read tunnel config: %+v%", tc)

	id := uint32(1) // test
	t := newTunnel(id, &tc)
	fmt.Println("create tunnel: ", t)
	return nil
}

func (l *Link) handleTunnelStream(frame *linkFrame) error {
	logrus.Debugf("handle tunnel stream: %s", frame)
	return nil
}

func (l *Link) handleTunnelClose(frame *linkFrame) error {
	logrus.Debugf("handle tunnel close: %s", frame)
	return nil
}

func mustRead(conn io.Reader, dlen uint32) ([]byte, error) {
	data := make([]byte, dlen)
	for i := 0; i < int(dlen); {
		n, err := conn.Read(data[i:])
		if err != nil {
			return nil, err
		}
		i += n
	}
	return data, nil
}
