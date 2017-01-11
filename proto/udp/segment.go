package udp

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

const (
	// protoVersion is the only version we support
	protoVersion uint8 = 0
	headerSize         = 30

	segTypeMsgSYN      uint8 = 1
	segTypeMsgACK      uint8 = 2
	segTypeMsgPingReq  uint8 = 3
	segTypeMsgPingRep  uint8 = 4
	segTypeMsgReceived uint8 = 5
	segTypeMsgReTrans  uint8 = 6
	segTypeMsgTrans    uint8 = 7

	segmentMaxSize     = 1400
	segmentBodyMaxSize = segmentMaxSize - headerSize // <= MTU

	handshakeKey = "ES HANDSHAKE" // TODO: use this
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

type segment struct {
	h header
	b []byte
}

func (seg *segment) bytes() []byte {
	return append(seg.h, seg.b...)
}

func (seg *segment) length() int {
	return headerSize + len(seg.b)
}

// | Version(1) | Type(1) | Flags(2) | StreamID(4) | TransID(2) | OrderID(2) | Checksum(16) | Length(2) |
func newSegment(segType uint8, flags uint16, streamID uint32, transID uint16, orderID uint16, message []byte) (*segment, error) {
	length := len(message)
	if length > segmentBodyMaxSize {
		return nil, errSegmentBodyTooLarge
	}
	hdr := header(make([]byte, headerSize))
	if message == nil {
		message = []byte{} // FIXME!
	}
	checksum := md5.Sum(message)
	hdr.encode(segType, flags, streamID, transID, orderID, checksum, uint16(length))
	return &segment{h: hdr, b: message}, nil
}

func newSYNSegment() *segment {
	seg, _ := newSegment(segTypeMsgSYN, 0, 0, 0, 0, []byte(handshakeKey))
	return seg
}

func newACKSegment(key []byte) *segment {
	seg, _ := newSegment(segTypeMsgACK, 0, 0, 0, 0, key)
	return seg
}

func newPingReqSegment(streamID uint32, id uint32) *segment {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, id)
	seg, _ := newSegment(segTypeMsgPingReq, 0, streamID, 0, 0, b)
	return seg
}

func newPingRepSegment(streamID uint32, b []byte) *segment {
	seg, _ := newSegment(segTypeMsgPingRep, 0, streamID, 0, 0, b)
	return seg
}

func newSingleSegment(segType uint8, flags uint16, streamID uint32, message []byte) *segment {
	hdr := header(make([]byte, headerSize))
	hdr.encode(segType, flags, streamID, 0, 0, md5.Sum(message), uint16(len(message)))
	return &segment{
		h: hdr,
		b: message,
	}
}

func loadSegment(data []byte) (*segment, error) {
	hdr := header(make([]byte, headerSize))
	copy(hdr, data[0:headerSize])
	seg := &segment{h: hdr, b: data[headerSize:]}
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
