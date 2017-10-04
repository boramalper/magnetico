package mainline

import (
	"bytes"
	"testing"
	"encoding/hex"
	"strings"
	"fmt"
)

func TestBEP33Filter(t *testing.T) {
	bf := new(BloomFilter)
	populateForBEP33(bf)

	resultingFilter := bf.Filter()
	var expectedFilter [256]byte
	hex.Decode(expectedFilter[:], []byte(strings.Replace(
		"F6C3F5EA A07FFD91 BDE89F77 7F26FB2B FF37BDB8 FB2BBAA2 FD3DDDE7 BACFFF75 EE7CCBAE" +
		"FE5EEDB1 FBFAFF67 F6ABFF5E 43DDBCA3 FD9B9FFD F4FFD3E9 DFF12D1B DF59DB53 DBE9FA5B" +
		"7FF3B8FD FCDE1AFB 8BEDD7BE 2F3EE71E BBBFE93B CDEEFE14 8246C2BC 5DBFF7E7 EFDCF24F" +
		"D8DC7ADF FD8FFFDF DDFFF7A4 BBEEDF5C B95CE81F C7FCFF1F F4FFFFDF E5F7FDCB B7FD79B3" +
		"FA1FC77B FE07FFF9 05B7B7FF C7FEFEFF E0B8370B B0CD3F5B 7F2BD93F EB4386CF DD6F7FD5" +
		"BFAF2E9E BFFFFEEC D67ADBF7 C67F17EF D5D75EBA 6FFEBA7F FF47A91E B1BFBB53 E8ABFB57" +
		"62ABE8FF 237279BF EFBFEEF5 FFC5FEBF DFE5ADFF ADFEE1FB 737FFFFB FD9F6AEF FEEE76B6" +
		"FD8F72EF",
	" ", "", -1)))
	if !bytes.Equal(resultingFilter[:], expectedFilter[:]) {
		t.Fail()
	}
}

func TestBEP33Estimation(t *testing.T) {
	bf := new(BloomFilter)
	populateForBEP33(bf)

	// Because Go lacks a truncate function for floats...
	if fmt.Sprintf("%.5f", bf.Estimate())[:9] != "1224.9308" {
		t.Errorf("Expected 1224.9308 got %f instead!", bf.Estimate())
	}
}

func populateForBEP33(bf *BloomFilter) {
	// 192.0.2.0 to 192.0.2.255 (both ranges inclusive)
	addr := []byte{192, 0, 2, 0}
	for i := 0; i <= 255; i++ {
		addr[3] = uint8(i)
		bf.InsertIP(addr)
	}

	// 2001:DB8:: to 2001:DB8::3E7 (both ranges inclusive)
	addr = []byte{32, 1, 13, 184, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	for i := 0; i <= 2; i++ {
		addr[14] = uint8(i)
		for e := 0; e <= 255; e++ {
			addr[15] = uint8(e)
			bf.InsertIP(addr)
		}
	}
	addr[14] = 3
	for e := 0; e <= 231; e++ {
		addr[15] = uint8(e)
		bf.InsertIP(addr)
	}
}
