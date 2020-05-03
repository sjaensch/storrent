package dht

import (
	"bytes"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/jackpal/bencode-go"
)

const maxNodesPerBucket = 8

var bootstrapNodes = []string{
	"router.utorrent.com:6881",
	"dht.transmissionbt.com:6881",
	"router.bittorrent.com:6881",
}

// DHT represents the DHT routing table
type DHT struct {
	NodeID     *[20]byte
	BucketTree *BucketTree
}

// BucketTree is an entry in the search tree for our routing table
type BucketTree struct {
	LeftChild  *BucketTree // left and right children; if unset then this is a leaf, use Bucket insteada
	RightChild *BucketTree
	RightLSB   byte // least significant bit that must be set for right child; use left child for smaller values
	Bucket     *Bucket
}

// Bucket represents one of the up to 160 buckets in the DHT, organized as a search tree
type Bucket struct {
	Min           byte // least significant bit for nodes in this bucket, starting count at 1
	Max           byte // most significant bit for nodes in this bucket
	Nodes         *Node
	Count         byte
	LastRefreshed time.Time
}

// Node in the DHT
type Node struct {
	Next    *Node
	ID      *[20]byte
	Address *net.UDPAddr
}

// BootstrapDHT initializes the DHT and fills it with the first nodes retrieved
// when looking for the given infohash
func BootstrapDHT(infohash []byte) (DHT, error) {
	dht := DHT{
		NodeID:     new([20]byte),
		BucketTree: &BucketTree{},
	}
	rand.Read(dht.NodeID[:])

	raddr, err := net.ResolveUDPAddr("udp", bootstrapNodes[0])
	if err != nil {
		return dht, err
	}
	bootstrapNode := Node{
		Address: raddr,
	}

	nodes, err := bootstrapNode.FindNode(dht.NodeID[:], infohash)
	if err != nil {
		return dht, err
	}
	dht.BucketTree.Bucket = &Bucket{
		Nodes: nodes,
	}

	return dht, nil
}

func (node *Node) FindNode(ourID, infohash []byte) (*Node, error) {
	conn, err := net.DialUDP("udp", nil, node.Address)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	query := NewKRPCFindNodeQuery(ourID, infohash)
	bencodeBytes, err := KRPCEncode(query)
	if err != nil {
		return nil, err
	}
	n, err := conn.Write(bencodeBytes)
	if err != nil {
		return nil, err
	}
	log.Printf("FindNode query bytes=%d data=%s", n, bencodeBytes)

	deadline := time.Now().Add(5 * time.Second)
	err = conn.SetReadDeadline(deadline)
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, 4096)
	nRead, addr, err := conn.ReadFrom(buffer)
	if err != nil {
		return nil, err
	}
	log.Printf("UDP packet received: bytes=%d from=%s data=%s", nRead, addr.String(), string(buffer))

	response := KRPCFindNodeResponse{}
	err = bencode.Unmarshal(bytes.NewReader(buffer), &response)
	if err != nil {
		return nil, err
	}

	count, node, err := response.toNodes()
	log.Printf("Got %d nodes in response", count)
	return node, nil
}
