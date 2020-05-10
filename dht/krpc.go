package dht

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"reflect"
	"time"

	"github.com/jackpal/bencode-go"
)

// KRPCMessage contains the basic fields that every message should have.
// Unfortunately I wasn't able to incorporate it within the more specific
// message types. Adding *KRPCMessage instead of duplicating the fields
// caused a panic when unmarshalling.
type KRPCMessage struct {
	TransactionID string `bencode:"t"` // Length: 2
	MessageType   string `bencode:"y"` // Length: 1
	ClientVersion string `bencode:"v"` // Length: 4
}

type KRPCQuery struct {
	*KRPCMessage
	QueryMethod string `bencode:"q"`
}

type KRPCFindNodeQuery struct {
	//*KRPCQuery
	TransactionID string                `bencode:"t"` // Length: 2
	MessageType   string                `bencode:"y"` // Length: 1
	ClientVersion string                `bencode:"v"` // Length: 4
	QueryMethod   string                `bencode:"q"`
	Arguments     KRPCFindNodeQueryArgs `bencode:"a"`
}

type KRPCFindNodeQueryArgs struct {
	NodeID       string `bencode:"id"`
	TargetNodeID string `bencode:"target"`
}

type KRPCFindNodeResponse struct {
	TransactionID string                   `bencode:"t"` // Length: 2
	MessageType   string                   `bencode:"y"` // Length: 1
	ClientVersion string                   `bencode:"v"` // Length: 4
	Arguments     KRPCFindNodeResponseArgs `bencode:"r"`
	Error         []interface{}            `bencode:"e"` // two items, error code (int) and error message
}

type KRPCFindNodeResponseArgs struct {
	NodeID string `bencode:"id"`
	Nodes  string `bencode:"nodes"`
}

// Request sends the given query to the node, putting the reply in response.
// query must be passed by value, response must be a pointer. This is a requirement
// by the bencode library. We have no type checking here, so we implement
// it manually.
func Request(node *Node, query, response interface{}) error {
	if reflect.ValueOf(query).Kind() != reflect.Struct {
		return fmt.Errorf("Need to pass query by value")
	}
	if reflect.ValueOf(response).Kind() != reflect.Ptr {
		return fmt.Errorf("Need to pass response as a pointer")
	}
	
	conn, err := net.DialUDP("udp", &net.UDPAddr{Port: 6881}, node.Address)
	if err != nil {
		return err
	}
	defer conn.Close()

	bencodeBytes, err := KRPCEncode(query)
	if err != nil {
		return err
	}
	n, err := conn.Write(bencodeBytes)
	if err != nil {
		return err
	}
	log.Printf("KRPC query bytes=%d data=%s", n, bencodeBytes)

	deadline := time.Now().Add(5 * time.Second)
	err = conn.SetReadDeadline(deadline)
	if err != nil {
		return err
	}

	buffer := make([]byte, 4096)
	nRead, addr, err := conn.ReadFrom(buffer)
	if err != nil {
		return err
	}
	log.Printf("UDP packet received: bytes=%d from=%s data=%s", nRead, addr.String(), string(buffer))

	err = bencode.Unmarshal(bytes.NewReader(buffer), response)
	return err
}

func (resp *KRPCFindNodeResponse) toNodes() (int, *Node, error) {
	if resp.MessageType == "e" {
		return 0, nil, fmt.Errorf("Error finding nodes: code=%d message=%s", resp.Error[0], resp.Error[1])
	}
	var first, cur *Node
	nodestr := resp.Arguments.Nodes
	count := len(nodestr) / 26
	for i := 0; i < count; i++ {
		new := Node{
			ID: new([20]byte), // will be filled below
			Address: &net.UDPAddr{
				// the IP we receive is in network byte order, and it's supposed to be in network byte order here as well
				IP:   []byte(nodestr[i*26+20 : i*26+24]),
				Port: int(binary.BigEndian.Uint16([]byte(nodestr[i*26+24 : i*26+26]))),
			},
			LastActive: time.Now(),
		}
		copy(new.ID[:], []byte(nodestr[i*26:i*26+20]))
		if cur == nil {
			first = &new
			cur = first
		} else {
			cur.Next = &new
			cur = &new
		}
	}

	return count, first, nil
}

func NewKRPCFindNodeQuery(source []byte, target []byte) KRPCFindNodeQuery {
	return KRPCFindNodeQuery{
		QueryMethod:   "find_node",
		TransactionID: "aa",
		MessageType:   "q",
		ClientVersion: "JT00",
		Arguments: KRPCFindNodeQueryArgs{
			NodeID:       string(source[:]),
			TargetNodeID: string(target[:]),
		},
	}
}

func KRPCEncode(query interface{}) ([]byte, error) {
	// We'll need to buffer the output by bencode.Marshal, and send it
	// in one write. If we let it write directly to conn it will create
	// many tiny UDP packets and it won't work.
	var bencodeBuf bytes.Buffer
	err := bencode.Marshal(&bencodeBuf, query)
	if err != nil {
		return nil, err
	}

	return bencodeBuf.Bytes(), nil
}
