package udp

import (
	"bytes"
	"container/heap"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
)

const (
	numRetransmit  = 9
	defaultTimeout = 100
	maxTimeout     = 1600

	defaultConnTranSize = 10
	defaultConnTimeout  = 30 * time.Second
	defaultPingInterval = 6 * time.Second
	defaultPingTimeout  = 3 * time.Second

	maxRecvPoolSize = 10
	maxSendPoolSize = 10
)

var (
	// ErrTimeout is commont timeout error
	ErrTimeout = errors.New("timeout")
	// ErrConnectionShutdown is a chan single of connection shutdown
	ErrConnectionShutdown  = errors.New("connection is shutdown")
	errSegmentChecksum     = errors.New("segment checksum error")
	errClientExist         = errors.New("client is exist in ClientPool")
	errSegmentBodyTooLarge = errors.New("segment body is too large")
)

type segmentHeap []*segment

func (h segmentHeap) Len() int           { return len(h) }
func (h segmentHeap) Less(i, j int) bool { return h[i].h.OrderID() < h[j].h.OrderID() }
func (h segmentHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *segmentHeap) Push(x interface{}) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*h = append(*h, x.(*segment))
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

func (m *msgRecving) Save(seg *segment) ([]byte, error) {
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
	segments []*segment
}

func newMsgSending() *msgSending {
	return &msgSending{}
}

func (m *msgSending) Save(seg *segment) error {
	return nil
}

// Conn is a UDP implement of es.Conn
type Conn struct {
	c          *net.UDPConn
	raddr      *net.UDPAddr
	id         uint32
	rl         []*msgRecving // recving list
	sl         []*msgSending // sending list
	lastActive time.Time
	inbound    chan []byte

	// pings is used to track inflight pings
	pings    map[uint32]chan struct{}
	pingID   uint32
	pingLock sync.Mutex

	shutdownCh chan struct{}
}

func newConn(conn *net.UDPConn, raddr *net.UDPAddr, id uint32) *Conn {
	return &Conn{
		c:          conn,
		raddr:      raddr,
		id:         id,
		rl:         make([]*msgRecving, defaultConnTranSize),
		sl:         make([]*msgSending, defaultConnTranSize),
		lastActive: time.Now(),
		inbound:    make(chan []byte, 1),

		pings: make(map[uint32]chan struct{}),

		shutdownCh: make(chan struct{}),
	}
}

// RemoteAddr get the address of remote endpoint
func (c *Conn) RemoteAddr() net.Addr {
	return c.raddr
}

// LocalAddr get the address of local endpoint
func (c *Conn) LocalAddr() net.Addr {
	return c.c.LocalAddr()
}

func (c *Conn) String() string {
	return fmt.Sprintf("conn %d: %s(L) -- %s(R)", c.id, c.LocalAddr(), c.RemoteAddr())
}

func (c *Conn) handle(msg []byte) error {
	c.lastActive = time.Now()

	seg, err := loadSegment(msg)
	if err != nil {
		return err
	}

	types := seg.h.Type()

	switch types {
	case segTypeMsgSYN:
		seg = newACKSegment(seg.b) // FIXME!
		_, err = c.c.WriteToUDP(seg.bytes(), c.raddr)
		return err
	case segTypeMsgPingReq:
		seg = newPingRepSegment(c.id, seg.b)
		_, err = c.c.WriteToUDP(seg.bytes(), c.raddr)
		return err
	case segTypeMsgPingRep:
		// notice ping wait
		pingID := binary.BigEndian.Uint32(seg.b[0:4])
		c.pingLock.Lock()
		ch := c.pings[pingID]
		if ch != nil {
			delete(c.pings, pingID)
			close(ch)
		}
		c.pingLock.Unlock()
		return nil
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
		seg, _ := newSegment(segTypeMsgReceived, 0, c.id, transID, 0, nil)
		c.c.WriteToUDP(seg.bytes(), c.raddr)
		return nil // FIXME! error
	}
	return nil
}

// RecvMsg recv a single message
func (c *Conn) RecvMsg() ([]byte, error) {
	// TODO: timeout
	msg := <-c.inbound
	return msg, nil
}

// SendMsg send a single message
func (c *Conn) SendMsg(message []byte) error {
	length := len(message)
	if length <= 0 {
		return errors.New("empty message")
	}

	// signle segment
	if length <= segmentBodyMaxSize {
		seg, _ := newSegment(segTypeMsgTrans, 0, c.id, 0, 0, message)
		_, err := c.c.WriteToUDP(seg.bytes(), c.raddr)
		return err
	}

	// multi segments

	// TODO: pack header
	multiHdr := make([]byte, 4)
	binary.BigEndian.PutUint32(multiHdr, uint32(length+4))
	message = append(multiHdr, message...)
	length = len(message)

	transID := uint16(0)
	for i := 0; i <= (length / segmentBodyMaxSize); i++ {
		end := (i + 1) * segmentBodyMaxSize
		if end > length {
			end = length
		}
		b := message[i*segmentBodyMaxSize : end]
		seg, _ := newSegment(segTypeMsgTrans, 0, c.id, transID, uint16(i+1), b)
		_, err := c.c.WriteToUDP(seg.bytes(), c.raddr)
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

// Ping is used to measure the RTT response time
func (c *Conn) Ping() (time.Duration, error) {
	ch := make(chan struct{})

	// Get a new ping id, mark as pending
	c.pingLock.Lock()
	id := c.pingID
	c.pingID++
	c.pings[id] = ch
	c.pingLock.Unlock()

	// Send the ping request
	seg := newPingReqSegment(c.id, id)
	c.c.WriteToUDP(seg.bytes(), c.raddr)

	// Wait for a response
	start := time.Now()
	select {
	case <-ch:
	case <-time.After(defaultPingTimeout):
		c.pingLock.Lock()
		delete(c.pings, id)
		c.pingLock.Unlock()
		return 0, ErrTimeout
	case <-c.shutdownCh:
		return 0, ErrConnectionShutdown
	}

	// TODO: compute time duration
	return time.Now().Sub(start), nil
}

// Close close this connection
func (c *Conn) Close() error {
	logrus.Warnf("close is not completed")
	return nil
}

// ConnPool manage all connections
type ConnPool struct {
	addrConnMap map[string]*Conn
	m           sync.Mutex
}

func newConnPool() *ConnPool {
	return &ConnPool{
		addrConnMap: map[string]*Conn{},
		m:           sync.Mutex{},
	}
}

// Get get the connection specified by address string
func (p *ConnPool) Get(addr net.Addr) (*Conn, bool) {
	p.m.Lock()
	c, ok := p.addrConnMap[addr.String()]
	p.m.Unlock()
	return c, ok
}

// New create a special single connection
func (p *ConnPool) New(conn *net.UDPConn, raddr *net.UDPAddr, id uint32) (*Conn, error) {
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

// Delete remove a conn from client pool
func (p *ConnPool) Delete(conn *Conn) error {
	p.m.Lock()
	defer p.m.Unlock()
	addr := conn.raddr.String()
	if _, ok := p.addrConnMap[addr]; !ok {
		return errors.New("delete: no such addr in addrConnMap")
	}
	delete(p.addrConnMap, addr)
	return nil
}

// GarbageCollection delete the disconnected clients
func (p *ConnPool) GarbageCollection() {
	addrs := []string{}
	p.m.Lock()
	for addr, conn := range p.addrConnMap {
		if time.Since(conn.lastActive) > defaultConnTimeout {
			addrs = append(addrs, addr)
		}
	}
	p.m.Unlock()

	p.m.Lock()
	for _, addr := range addrs {
		delete(p.addrConnMap, addr)
		logrus.Debugf("client %s is timeout, delete it", addr)
	}
	p.m.Unlock()
}

type udpserver struct {
	c *net.UDPConn

	curClientID uint32
	idAddrPool  map[uint32]string
	idm         sync.Mutex

	connPool *ConnPool
	clientCh chan *Conn
}

func (p *udpserver) newClientID() uint32 {
	p.idm.Lock()
	defer p.idm.Unlock()
	for {
		p.curClientID++
		if _, ok := p.idAddrPool[p.curClientID]; !ok {
			return p.curClientID
		}
	}
}

func (p *udpserver) garbageCollection() {
	for {
		start := time.Now()
		p.connPool.GarbageCollection()
		time.Sleep(10*time.Second - time.Now().Sub(start))
	}
}

func (p *udpserver) recv() error {
	// FIXME!
	go p.garbageCollection()

	buf := make([]byte, segmentMaxSize)
	for {
		n, raddr, err := p.c.ReadFromUDP(buf)
		// logrus.Info("Read: n, raddr, err = ", n, raddr, err)
		// fmt.Println("\n" + hex.Dump(buf[0:n]))
		if err != nil {
			logrus.Errorf("ReadFromUDP error: %s", err)
			return err
		}

		conn, ok := p.connPool.Get(raddr)
		if !ok {
			// save new client
			id := p.newClientID()
			conn, err = p.connPool.New(p.c, raddr, id)
			if err != nil {
				logrus.Errorf("save new client failed: %s", err)
				// TODO: notice schema
				seg := newACKSegment([]byte("error: create client conn"))
				p.c.WriteToUDP(seg.bytes(), raddr)
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

// Accept wait the new client connection incoming
func (p *udpserver) Accept() (*Conn, error) {
	return <-p.clientCh, nil
}

// ClientSocket is a UDP implement of Socket
type ClientSocket struct {
	udpserver
	raddr *net.UDPAddr
}

// NewClientSocket create a client socket
func NewClientSocket(conn *net.UDPConn, raddr *net.UDPAddr) (*ClientSocket, *Conn, error) {
	sock := &ClientSocket{
		udpserver: udpserver{
			c:        conn,
			connPool: newConnPool(),
			clientCh: make(chan *Conn, 1),
		},
		raddr: raddr,
	}
	c, err := sock.handshake() // FIXME! quit?
	if err != nil {
		return nil, nil, err
	}
	go sock.pingLoop(c)
	go sock.recv()
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
	// send heartbeat and wait
	seg := newSYNSegment()
	_, err := p.c.WriteToUDP(seg.bytes(), p.raddr)
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
	if string(seg.b) != handshakeKey {
		logrus.Warnf("handshake: response segment body is mismatch")
		return nil, errors.New("response segment body is mismatch")
	}

	// TODO: check streamID
	return p.connPool.New(p.c, p.raddr, seg.h.StreamID())
}

func (p *ClientSocket) pingLoop(c *Conn) {
	for {
		c.Ping() // ping timeout
		time.Sleep(defaultPingInterval)
	}
}

// ServerSocket is a UDP implement of socket
type ServerSocket struct {
	udpserver
}

// NewServerSocket create a UDPConn
func NewServerSocket(conn *net.UDPConn) (*ServerSocket, error) {
	sock := &ServerSocket{
		udpserver{
			c:        conn,
			connPool: newConnPool(),
			clientCh: make(chan *Conn, 1),
		},
	}
	go sock.recv()
	return sock, nil
}
