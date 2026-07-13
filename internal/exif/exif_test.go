package exif

import (
	"encoding/binary"
	"testing"
)

func TestParseJPEGReadsBasicCameraFields(t *testing.T) {
	tiff := make([]byte, 80)
	copy(tiff[:2], "II")
	binary.LittleEndian.PutUint16(tiff[2:4], 42)
	binary.LittleEndian.PutUint32(tiff[4:8], 8)
	binary.LittleEndian.PutUint16(tiff[8:10], 3)
	putEntry := func(offset int, tag, kind uint16, count, value uint32) {
		binary.LittleEndian.PutUint16(tiff[offset:offset+2], tag)
		binary.LittleEndian.PutUint16(tiff[offset+2:offset+4], kind)
		binary.LittleEndian.PutUint32(tiff[offset+4:offset+8], count)
		binary.LittleEndian.PutUint32(tiff[offset+8:offset+12], value)
	}
	putEntry(10, 0x010f, 2, 6, 50)
	putEntry(22, 0x0110, 2, 7, 56)
	putEntry(34, 0x0112, 3, 1, 1)
	copy(tiff[50:], "Canon\x00")
	copy(tiff[56:], "EOS R6\x00")
	segment := append([]byte("Exif\x00\x00"), tiff...)
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe1, byte((len(segment) + 2) >> 8), byte(len(segment) + 2)}
	jpeg = append(jpeg, segment...)
	metadata, err := ParseJPEG(jpeg)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Manufacturer != "Canon" || metadata.Camera != "EOS R6" || metadata.Orientation != 1 {
		t.Fatalf("unexpected metadata: %+v", metadata)
	}
}
