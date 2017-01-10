package udp

import (
	"bytes"
	"container/heap"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
)

const (
	// protoVersion is the only version we support
	protoVersion uint8 = 0
	headerSize         = 30

	segTypeMsgSYN      uint8 = 1
	segTypeMsgACK      uint8 = 2
	segTypeMsgReceived uint8 = 3
	segTypeMsgReTrans  uint8 = 4
	segTypeMsgTrans    uint8 = 5
)

const (
	segmentMaxSize     = 1400
	segmentBodyMaxSize = segmentMaxSize - headerSize // <= MTU

	handshakeKey   = "handshake"
	numRetransmit  = 9
	defaultTimeout = 100
	maxTimeout     = 1600

	defaultConnTranSize = 10

	maxRecvPoolSize = 10
	maxSendPoolSize = 10
)

const (
	// SYN is sent to signal a new stream. May
	// be sent with a data payload
	flagSYN uint16 = 1 << iota

	// ACK is sent to acknowledge a new stream. May
	// be sent with a data payload
	flagACK

	// FIN is sent to half-close the given stream.
	// May be sent with a data payload.
	flagFIN

	// RST is used to hard close a given stream.
	flagRST
)

var (
	errSegmentChecksum = errors.New("segment checksum error")
	errClientExist     = errors.New("client is exist in ClientPool")
)

// segment header
// | Version(1) | Type(1) | Flags(2) | StreamID(4) | TransID(2) | OrderID(2) | Checksum(16) | Length(2) |
type header []byte

func (h header) Version() uint8 {
	return h[0]
}
func (h header) Type() uint8 {
	return h[1]
}
func (h header) Flags() uint16 {
	return binary.BigEndian.Uint16(h[2:4])
}
func (h header) StreamID() uint32 {
	return binary.BigEndian.Uint32(h[4:8])
}
func (h header) TransID() uint16 {
	return binary.BigEndian.Uint16(h[8:10])
}
func (h header) OrderID() uint16 {
	return binary.BigEndian.Uint16(h[10:12])
}
func (h header) Checksum() []byte {
	return h[12:28]
}
func (h header) Length() uint16 {
	return binary.BigEndian.Uint16(h[28:30])
}
func (h header) String() string {
	return fmt.Sprintf("Version:%d Type:%d Flags:%d StreamID:%d TransID:%d OrderID:%d Length:%d Checksum:%s",
		h.Version(), h.Type(), h.Flags(), h.StreamID(), h.TransID(), h.OrderID(), h.Length(), hex.EncodeToString(h.Checksum()))
}
func (h header) encode(segType uint8, flags uint16, streamID uint32, transID uint16, orderID uint16, checksum [md5.Size]byte, length uint16) {
	h[0] = protoVersion
	h[1] = segType
	binary.BigEndian.PutUint16(h[2:4], flags)
	binary.BigEndian.PutUint32(h[4:8], streamID)
	binary.BigEndian.PutUint16(h[8:10], transID)
	binary.BigEndian.PutUint16(h[10:12], orderID)
	copy(h[12:28], checksum[:])
	binary.BigEndian.PutUint16(h[28:30], length)
}

type Segment struct {
	h header
	b []byte
}

func (seg *Segment) Bytes() []byte {
	return append(seg.h, seg.b...)
}

func (seg *Segment) Length() int {
	return headerSize + len(seg.b)
}

func newSingleSegment(segType uint8, flags uint16, streamID uint32, message []byte) *Segment {
	hdr := header(make([]byte, headerSize))
	hdr.encode(segType, flags, streamID, 0, 0, md5.Sum(message), uint16(len(message)))
	return &Segment{
		h: hdr,
		b: message,
	}
}

func loadSegment(data []byte) (*Segment, error) {
	hdr := header(make([]byte, headerSize))
	copy(hdr, data[0:headerSize])
	seg := &Segment{h: hdr, b: data[headerSize:]}
	if hdr.Length() == 0 {
		return seg, nil // FIXME!
	}
	checksum := md5.Sum(seg.b)
	// FIXME!
	if !bytes.Equal(seg.h.Checksum(), checksum[:]) {
		return nil, errSegmentChecksum
	}
	return seg, nil
}

type segmentHeap []*Segment

func (h segmentHeap) Len() int           { return len(h) }
func (h segmentHeap) Less(i, j int) bool { return h[i].h.OrderID() < h[j].h.OrderID() }
func (h segmentHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *segmentHeap) Push(x interface{}) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*h = append(*h, x.(*Segment))
}

func (h *segmentHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type msgRecving struct {
	readBuf    bytes.Buffer
	nid        uint16 // next segment id, start by 1
	needLength uint32
	readLength uint32
	sh         *segmentHeap
	buf        bytes.Buffer
}

func newMsgRecving() *msgRecving {
	m := &msgRecving{
		nid: 1,
		sh:  &segmentHeap{},
	}
	heap.Init(m.sh)
	return m
}

func (m *msgRecving) Save(seg *Segment) ([]byte, error) {
	oid := seg.h.OrderID()
	if oid == m.nid {
		if oid == 1 {
			// FIXME!
			m.needLength = binary.BigEndian.Uint32(seg.b[0:4])
			m.readBuf.Write(seg.b[4:])
		} else {
			m.readBuf.Write(seg.b)
		}
		m.nid++
	} else {
		heap.Push(m.sh, seg)
	}

	m.readLength += uint32(len(seg.b))

	// FIXME: need the every seg.len ?
	if m.needLength > 0 && m.needLength == m.readLength {
		// read message completed
		for _, seg := range *m.sh {
			m.readBuf.Write(seg.b)
		}
		// TODO: cleanup ?
		return m.readBuf.Bytes(), nil
	}

	return nil, nil
}

type msgSending struct {
	segments []*Segment
}

func newMsgSending() *msgSending {
	return &msgSending{}
}

func (m *msgSending) Save(seg *Segment) error {
	return nil
}

// Conn is a UDP implement of es.Conn
type Conn struct {
	c       *net.UDPConn
	raddr   *net.UDPAddr
	id      uint32
	rl      []*msgRecving // recving list
	sl      []*msgSending // sending list
	inbound chan []byte
}

func newConn(conn *net.UDPConn, raddr *net.UDPAddr, id uint32) *Conn {
	return &Conn{
		c:       conn,
		raddr:   raddr,
		id:      id,
		rl:      make([]*msgRecving, defaultConnTranSize),
		sl:      make([]*msgSending, defaultConnTranSize),
		inbound: make(chan []byte, 1),
	}
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.raddr
}

func (c *Conn) LocalAddr() net.Addr {
	return c.c.LocalAddr()
}

func (c *Conn) String() string {
	return fmt.Sprintf("conn %d: %s(L) -- %s(R)", c.id, c.LocalAddr(), c.RemoteAddr())
}

func (c *Conn) handle(msg []byte) error {
	seg, err := loadSegment(msg)
	if err != nil {
		return err
	}

	types := seg.h.Type()

	switch types {
	case segTypeMsgSYN:
		seg = newSingleSegment(segTypeMsgACK, 0, c.id, []byte("handshake"))
		_, err = c.c.WriteToUDP(seg.Bytes(), c.raddr)
		return err
	case segTypeMsgReceived:
		// FIXME!
		transID := seg.h.TransID()
		if c.sl[transID] != nil {
			c.sl[transID] = nil
		} else {
			logrus.Warnf("no such transID: %d", transID)
		}
		return nil
	case segTypeMsgReTrans:
		logrus.Errorf("re trans message have not completed!")
	}

	// FIXME!
	if seg.h.OrderID() == 0 {
		// signle segment msg
		c.inbound <- seg.b
		return nil
	}
	transID := seg.h.TransID()
	recving := c.rl[transID]
	if recving == nil {
		recving = newMsgRecving()
		c.rl[transID] = recving
	}
	msg, err = recving.Save(seg)
	if err != nil {
		return err
	}
	if msg != nil {
		c.inbound <- msg
		// cleanup
		c.rl[transID] = nil
		// send msg received
		seg := &Segment{
			h: header(make([]byte, headerSize)),
			b: []byte{},
		}
		seg.h.encode(segTypeMsgReceived, 0, c.id, transID, 0, [md5.Size]byte{}, 0)
		c.c.WriteToUDP(seg.Bytes(), c.raddr)
		return nil // FIXME! error
	}
	return nil
}

func (c *Conn) RecvMsg() ([]byte, error) {
	// TODO: timeout
	msg := <-c.inbound
	return msg, nil
}

func (c *Conn) SendMsg(message []byte) error {
	length := len(message)
	if length <= 0 {
		return errors.New("empty message")
	}

	// signle segment
	if length <= segmentBodyMaxSize {
		seg := &Segment{
			h: header(make([]byte, headerSize)),
			b: message,
		}
		seg.h.encode(segTypeMsgTrans, 0, c.id, 0, 0, md5.Sum(message), uint16(length))
		_, err := c.c.WriteToUDP(seg.Bytes(), c.raddr)
		return err
	}

	// multi segments

	// TODO: pack header
	multiHdr := make([]byte, 4)
	binary.BigEndian.PutUint32(multiHdr, uint32(length+4))
	message = append(multiHdr, message...)
	length = len(message)

	transID := uint16(0)
	hdr := header(make([]byte, headerSize))
	for i := 0; i <= (length / segmentBodyMaxSize); i++ {
		end := (i + 1) * segmentBodyMaxSize
		if end > length {
			end = length
		}
		b := message[i*segmentBodyMaxSize : end]
		hdr.encode(segTypeMsgTrans, 0, c.id, transID, uint16(i+1), md5.Sum(b), uint16(len(b)))
		seg := &Segment{
			h: hdr,
			b: b,
		}
		_, err := c.c.WriteToUDP(seg.Bytes(), c.raddr)
		if err != nil {
			return err
		}
		// save to sending
		sending := c.sl[transID]
		if sending == nil {
			sending = newMsgSending()
			c.sl[transID] = sending
		}
		if err := sending.Save(seg); err != nil {
			// cleanup
			c.sl[transID] = nil
			return err
		}
	}
	return nil
}

func (c *Conn) Close() error {
	logrus.Warnf("close is not completed")
	return nil
}

// ClientPool manage all clients
// TODO: max clients limit!
type ClientPool struct {
	// id manager
	nextID uint32
	idm    sync.Mutex

	// pipes manager
	idAddrMap   map[uint32]string
	addrConnMap map[string]*Conn
	m           sync.Mutex
}

func newClientPool() *ClientPool {
	return &ClientPool{
		nextID:      0,
		idm:         sync.Mutex{},
		idAddrMap:   map[uint32]string{},
		addrConnMap: map[string]*Conn{},
		m:           sync.Mutex{},
	}
}

func (p *ClientPool) newID() uint32 {
	// TODO: lock
	p.idm.Lock()
	defer p.idm.Unlock()
	for {
		p.nextID++
		p.m.Lock()
		_, ok := p.idAddrMap[p.nextID]
		p.m.Unlock()
		if !ok {
			break
		}
	}
	return p.nextID
}

func (p *ClientPool) GetConn(addr string) (*Conn, bool) {
	p.m.Lock()
	c, ok := p.addrConnMap[addr]
	p.m.Unlock()
	return c, ok
}

// TODO: delete
func (p *ClientPool) NewSingle(conn *net.UDPConn, raddr *net.UDPAddr, id uint32) (*Conn, error) {
	addr := raddr.String()
	p.m.Lock()
	_, ok := p.addrConnMap[addr]
	p.m.Unlock()
	if ok {
		return nil, errClientExist
	}
	c := newConn(conn, raddr, id)
	p.m.Lock()
	p.addrConnMap[addr] = c
	p.m.Unlock()
	return c, nil
}

func (p *ClientPool) New(conn *net.UDPConn, raddr *net.UDPAddr) (*Conn, error) {
	addr := raddr.String()
	p.m.Lock()
	_, ok := p.addrConnMap[addr]
	p.m.Unlock()
	if ok {
		return nil, errClientExist
	}
	id := p.newID()
	c := newConn(conn, raddr, id)
	p.m.Lock()
	p.idAddrMap[id] = addr
	p.addrConnMap[addr] = c
	p.m.Unlock()
	return c, nil
}

func (p *ClientPool) Delete(conn *Conn) error {
	p.m.Lock()
	defer p.m.Unlock()
	addr, ok := p.idAddrMap[conn.id]
	if !ok {
		return errors.New("delete: no such id in idAddrMap")
	}
	_, ok = p.addrConnMap[addr]
	if !ok {
		return errors.New("delete: no such addr in addrConnMap")
	}
	delete(p.addrConnMap, addr)
	delete(p.idAddrMap, conn.id)
	return nil
}

// ClientSocket is a UDP implement of Socket
type ClientSocket struct {
	c        *net.UDPConn
	raddr    *net.UDPAddr
	cp       *ClientPool
	clientCh chan *Conn
}

// NewClientSocket create a client socket
func NewClientSocket(conn *net.UDPConn, raddr *net.UDPAddr) (*ClientSocket, *Conn, error) {
	sock := &ClientSocket{
		c:        conn,
		raddr:    raddr,
		cp:       newClientPool(),
		clientCh: make(chan *Conn, 1),
	}
	c, err := sock.handshake() // FIXME! quit?
	if err != nil {
		return nil, nil, err
	}
	go sock.recvLoop()
	return sock, c, nil
}

func (p *ClientSocket) handshake() (*Conn, error) {
	for {
		if conn, err := p._handshake(); err == nil {
			return conn, err
		}
		time.Sleep(6 * time.Second)
	}
}

func (p *ClientSocket) _handshake() (*Conn, error) {
	key := []byte("handshake")

	// send heartbeat and wait
	seg := &Segment{
		h: header(make([]byte, headerSize)),
		b: key,
	}
	seg.h.encode(segTypeMsgSYN, 0, 0, 0, 0, md5.Sum(key), uint16(len(key)))
	_, err := p.c.WriteToUDP(seg.Bytes(), p.raddr)
	if err != nil {
		logrus.Warnf("handshake: write segment failed: %s", err)
		return nil, err
	}

	buf := make([]byte, segmentBodyMaxSize)

	// read
	n, raddr, err := p.c.ReadFromUDP(buf)
	if raddr.String() != p.raddr.String() {
		logrus.Warnf("unknown from addr: %s", raddr.String())
	}
	if err != nil {
		logrus.Warnf("handshake: read segment failed: %s", err)
		return nil, err
	}

	seg, err = loadSegment(buf[0:n])
	if err != nil {
		logrus.Warnf("handshake: loadSegment failed: %s", err)
		return nil, err
	}
	if seg.h.Type() != segTypeMsgACK {
		logrus.Warnf("handshake: segment type is %d, not segTypeMsgSYN(%d)", seg.h.Type(), segTypeMsgSYN)
		return nil, errors.New("segment type is not segTypeMsgACK")
	}
	if !bytes.Equal(seg.b, key) {
		logrus.Warnf("handshake: response segment body is mismatch")
		return nil, errors.New("response segment body is mismatch")
	}

	// TODO:
	return p.cp.NewSingle(p.c, p.raddr, seg.h.StreamID())
}

func (p *ClientSocket) recvLoop() error {
	buf := make([]byte, segmentMaxSize)
	for {
		n, raddr, err := p.c.ReadFromUDP(buf)
		// logrus.Info("Read: n, raddr, err = ", n, raddr, err)
		// fmt.Println("\n" + hex.Dump(buf[0:n]))
		if err != nil {
			return err
		}

		conn, ok := p.cp.GetConn(raddr.String())
		if !ok {
			// new client incoming
			conn, err = p.cp.New(p.c, raddr)
			if err != nil {
				logrus.Errorf("create new client conn failed: %s", err)
				// notice remote endpoint
				res := []byte("error: create client conn")
				seg := &Segment{
					h: header(make([]byte, headerSize)),
					b: res,
				}
				seg.h.encode(segTypeMsgACK, 0, 0, 0, 0, md5.Sum(res), uint16(len(res)))
				p.c.WriteToUDP(seg.Bytes(), raddr)
				continue
			}
			p.clientCh <- conn
		}

		// handle in
		if err := conn.handle(buf[0:n]); err != nil {
			logrus.Errorf("handle msg(from %s) failed: %s", raddr.String(), err)
		}
	}
}

func (p *ClientSocket) Accept() (*Conn, error) {
	return <-p.clientCh, nil
}

// ServerSocket is a UDP implement of socket
type ServerSocket struct {
	c        *net.UDPConn
	cp       *ClientPool
	clientCh chan *Conn
}

// NewServerSocket create a UDPConn
func NewServerSocket(conn *net.UDPConn) (*ServerSocket, error) {
	sock := &ServerSocket{
		c:        conn,
		cp:       newClientPool(),
		clientCh: make(chan *Conn, 1),
	}
	go sock.recvLoop()
	return sock, nil
}

func (p *ServerSocket) recvLoop() error {
	buf := make([]byte, segmentMaxSize)
	for {
		n, raddr, err := p.c.ReadFromUDP(buf)
		// logrus.Info("Read segment: n, p.raddr, err = ", n, raddr, err)
		// fmt.Println("\n" + hex.Dump(buf[0:n]))
		if err != nil {
			return err
		}

		conn, ok := p.cp.GetConn(raddr.String())
		if !ok {
			// new client incoming
			conn, err = p.cp.New(p.c, raddr)
			if err != nil {
				logrus.Errorf("create new client conn failed: %s", err)
				// notice remote endpoint
				res := []byte("error: create client conn")
				seg := &Segment{
					h: header(make([]byte, headerSize)),
					b: res,
				}
				seg.h.encode(segTypeMsgACK, 0, 0, 0, 0, md5.Sum(res), uint16(len(res)))
				p.c.WriteToUDP(seg.Bytes(), raddr)
				continue
			}
			p.clientCh <- conn
		}

		// handle in
		if err := conn.handle(buf[0:n]); err != nil {
			logrus.Errorf("handle msg(from %s) failed: %s", raddr.String(), err)
		}
	}
}

func (p *ServerSocket) Accept() (*Conn, error) {
	return <-p.clientCh, nil
}
