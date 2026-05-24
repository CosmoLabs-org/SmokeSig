package runner

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// CheckMongoPing sends an isMaster command to a MongoDB server and verifies a valid response.
func CheckMongoPing(check *schema.MongoCheck) AssertionResult {
	host := check.Host
	if host == "" {
		host = "localhost"
	}
	port := check.Port
	if port == 0 {
		port = 27017
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return AssertionResult{Type: "mongo_ping", Expected: addr, Actual: err.Error(), Passed: false}
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	// MongoDB OP_QUERY message for isMaster on admin.$cmd
	// Header: int32 length, int32 requestID, int32 responseTo, int32 opCode=2004
	// Body: int32 flags(0), cstring "admin.$cmd", int32 numberToSkip(0), int32 numberToReturn(1)
	// BSON document: {isMaster: 1}

	bsonDoc := buildIsMasterBSON()

	// cstring "admin.$cmd" (null-terminated)
	collection := []byte("admin.$cmd\x00")

	// Body: flags(4) + collection + skip(4) + return(4) + bson
	bodyLen := 4 + len(collection) + 4 + 4 + len(bsonDoc)
	totalLen := 16 + bodyLen // header + body

	msg := make([]byte, totalLen)
	binary.LittleEndian.PutUint32(msg[0:4], uint32(totalLen))  // message length
	binary.LittleEndian.PutUint32(msg[4:8], 1)                 // requestID
	binary.LittleEndian.PutUint32(msg[8:12], 0)                // responseTo
	binary.LittleEndian.PutUint32(msg[12:16], 2004)            // OP_QUERY
	binary.LittleEndian.PutUint32(msg[16:20], 0)               // flags
	offset := 20
	copy(msg[offset:], collection)
	offset += len(collection)
	binary.LittleEndian.PutUint32(msg[offset:offset+4], 0)     // numberToSkip
	offset += 4
	binary.LittleEndian.PutUint32(msg[offset:offset+4], 1)     // numberToReturn (-1 = single doc)
	offset += 4
	copy(msg[offset:], bsonDoc)

	if _, err := conn.Write(msg); err != nil {
		return AssertionResult{Type: "mongo_ping", Expected: addr, Actual: "write error: " + err.Error(), Passed: false}
	}

	// Read response header (16 bytes)
	respHdr := make([]byte, 16)
	n, err := conn.Read(respHdr)
	if err != nil || n < 16 {
		return AssertionResult{Type: "mongo_ping", Expected: addr, Actual: fmt.Sprintf("read error: %v (n=%d)", err, n), Passed: false}
	}

	respLen := binary.LittleEndian.Uint32(respHdr[0:4])
	opCode := binary.LittleEndian.Uint32(respHdr[12:16])

	if opCode != 1 { // OP_REPLY
		return AssertionResult{Type: "mongo_ping", Expected: "OP_REPLY (1)", Actual: fmt.Sprintf("opCode=%d", opCode), Passed: false}
	}

	// Read remaining response
	remaining := int(respLen) - 16
	if remaining > 0 {
		respBody := make([]byte, remaining)
		n, err := conn.Read(respBody)
		if err != nil || n < remaining {
			return AssertionResult{Type: "mongo_ping", Expected: addr, Actual: fmt.Sprintf("incomplete response: %d/%d bytes", n, remaining), Passed: false}
		}
	}

	return AssertionResult{Type: "mongo_ping", Expected: addr, Actual: "isMaster OK", Passed: true}
}

// buildIsMasterBSON constructs a minimal BSON document: { "isMaster": 1 }
func buildIsMasterBSON() []byte {
	// BSON: int32 doc_size, type(0x01 for double), cstring "isMaster", float64(1.0), 0x00 terminator
	key := []byte("isMaster\x00")
	docSize := 4 + 1 + len(key) + 8 + 1 // size + type + key + float64 + terminator

	doc := make([]byte, docSize)
	binary.LittleEndian.PutUint32(doc[0:4], uint32(docSize))
	doc[4] = 0x01 // double type
	copy(doc[5:], key)
	// float64 1.0 in little-endian = 0x00 0x00 0x00 0x00 0x00 0x00 0xF0 0x3F
	binary.LittleEndian.PutUint64(doc[5+len(key):5+len(key)+8], 0x3FF0000000000000)
	doc[docSize-1] = 0x00 // terminator

	return doc
}
