package udp

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"testing"
)

const (
	maxMsgSize = 1024 * 1024 * 4
)

func Test_msgSending_IterBufferd(t *testing.T) {
	for length := 2; length <= maxMsgSize; length *= 2 {
		b := make([]byte, length+1)
		rand.Read(b)
		sending := newMsgSending(0, 0, 0, 0, b)
		var buf bytes.Buffer
		for seg := range sending.IterBufferd() {
			if seg.h.OrderID() == 0 {
				buf.Write(seg.b[4:])
			} else {
				buf.Write(seg.b)
			}
		}
		if !bytes.Equal(b, buf.Bytes()) {
			t.Errorf("IterBufferd mismatch")
		}
	}
}

func Test_msgSending_GetSegmentByOrderID(t *testing.T) {
	for length := 2; length <= maxMsgSize; length *= 2 {
		b := make([]byte, length+1)
		rand.Read(b)

		sending := newMsgSending(0, 0, 0, 0, b)
		sl := map[uint16]string{}
		for seg := range sending.IterBufferd() {
			sl[seg.h.OrderID()] = hex.EncodeToString(seg.h.Checksum())
		}
		for orderID, origCk := range sl {
			seg := sending.GetSegmentByOrderID(orderID)
			newCk := hex.EncodeToString(seg.h.Checksum())
			if newCk != origCk {
				t.Errorf("GetSegmentByOrderID mismatch: orderID = %d, orig ck = %s, new ck = %s", orderID, origCk, newCk)
			}
		}
	}
}

func Test_msgRecving_Save(t *testing.T) {
	for length := 2; length <= maxMsgSize; length *= 2 {
		b := make([]byte, length+1)
		rand.Read(b)

		sending := newMsgSending(0, 0, 0, 0, b)
		sl := map[uint16]*segment{} // map for random select
		for seg := range sending.IterBufferd() {
			sl[seg.h.OrderID()] = seg
		}

		recving := newMsgRecving()
		func() {
			for _, seg := range sl {
				msg, err := recving.Save(seg)
				if err != nil {
					t.Errorf("Save failed: %s", err)
				}
				if msg != nil {
					if !bytes.Equal(msg, b) {
						t.Errorf("recving msg mismatch: length = %d", length)
					}
					return // break inner for
				}
			}
			t.Errorf("recving not completed!")
		}()
	}
}
