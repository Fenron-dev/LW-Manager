package scanner

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestHEIFDimensions(t *testing.T) {
	ispe := make([]byte, 20)
	binary.BigEndian.PutUint32(ispe[0:4], uint32(len(ispe)))
	copy(ispe[4:8], "ispe")
	binary.BigEndian.PutUint32(ispe[12:16], 4032)
	binary.BigEndian.PutUint32(ispe[16:20], 3024)
	ipco := append(makeBox("ipco", nil), ispe...)
	binary.BigEndian.PutUint32(ipco[0:4], uint32(len(ipco)))
	iprp := makeBox("iprp", ipco)
	meta := makeBox("meta", append(make([]byte, 4), iprp...))
	width, height, err := heifDimensions(bytes.NewReader(meta))
	if err != nil || width != 4032 || height != 3024 {
		t.Fatalf("dimensions = %dx%d, %v", width, height, err)
	}
}

func makeBox(boxType string, payload []byte) []byte {
	box := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(box[:4], uint32(len(box)))
	copy(box[4:8], boxType)
	copy(box[8:], payload)
	return box
}
