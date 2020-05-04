package dht

import (
	"bytes"
	"crypto/rand"
	"log"
	"net"
	"time"

	"github.com/jackpal/bencode-go"

	"github.com/sjaensch/storrent/helpers"
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

// BucketTree is an entry in the binary tree for our routing table
type BucketTree struct {
	LeftChild  *BucketTree // left and right children; if unset then this is a leaf, use Bucket instead
	RightChild *BucketTree
	Level      byte // Level within the tree, starts at 0
	Bucket     *Bucket
}

// Bucket represents one of the up to 160 buckets in the DHT, organized as a search tree
type Bucket struct {
	Nodes         *Node
	Count         byte
	LastRefreshed time.Time
}

// Node in the DHT
type Node struct {
	Next       *Node
	ID         *[20]byte
	Address    *net.UDPAddr
	LastActive time.Time
}

// BootstrapDHT initializes the DHT and fills it with the first nodes retrieved
// when looking for the given infohash
func BootstrapDHT(infohash []byte) (DHT, error) {
	dht := DHT{
		NodeID: new([20]byte),
		BucketTree: &BucketTree{
			Level:  0,
			Bucket: &Bucket{},
		},
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

// InsertNode adds a Node to our routing table, potentially rebalancing the tree if necessary.
func (dht *DHT) InsertNode(node *Node) error {
	bitIndex := 0
	bucketTree := dht.BucketTree
	var bit byte

	for ; bucketTree.Bucket == nil; bitIndex++ {
		// if Bucket is nil then we need to have LeftChild and RightChild
		helpers.Assert(bucketTree.LeftChild != nil && bucketTree.RightChild != nil)

		bit = (node.ID[bitIndex/8] >> (7 - (bitIndex % 8))) & 1
		helpers.Assert(bit == 0 || bit == 1)
		if bit == 0 {
			bucketTree = bucketTree.LeftChild
		} else {
			bucketTree = bucketTree.RightChild
		}
	}

	if bucketTree.Bucket.Count < 8 {
		node.Next = bucketTree.Bucket.Nodes
		bucketTree.Bucket.Nodes = node
		bucketTree.Bucket.Count++
		bucketTree.Bucket.LastRefreshed = time.Now()
		return nil
	}

	// check whether all nodes are "good"
	var last, cur *Node
	for cur = bucketTree.Bucket.Nodes; cur != nil && cur.LastActive.Add(15*time.Minute).After(time.Now()); cur = cur.Next {
		last = cur
		cur = cur.Next
	}
	if cur != nil { // TODO
		removeNode
		addNode
	}

	// The prefix matches our ID: we split the bucket
	// Otherwise: discard the node
	return nil
}

func (node *Node) FindNode(ourID, infohash []byte) (*Node, error) {
	conn, err := net.DialUDP("udp", &net.UDPAddr{Port: 6881}, node.Address)
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
