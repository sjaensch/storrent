package dht

import (
	"bytes"
	"crypto/rand"
	"log"
	"net"
	"time"

	"github.com/sjaensch/storrent/err"
)

const maxNodesPerBucket = 8
const activePeriod = 15 * time.Minute

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
		err.Assert(bucketTree.LeftChild != nil && bucketTree.RightChild != nil)

		bit = (node.ID[bitIndex/8] >> (7 - (bitIndex % 8))) & 1
		err.Assert(bit == 0 || bit == 1)
		if bit == 0 {
			bucketTree = bucketTree.LeftChild
		} else {
			bucketTree = bucketTree.RightChild
		}
	}

	if bucketTree.Bucket.Count < 8 || bucketTree.Bucket.makeRoom() || prefixMatch(dht.NodeID[:], node.ID[:], bitIndex) {
		bucketTree.addNode(node)
	} else {
		log.Printf("Not inserting node, bucket is full.")
	}

	return nil
}

// Internal function that will add the Node to bucket, splitting it if necessary.
func (bucketTree *BucketTree) addNode(node *Node) {
	err.Assert(bucketTree.Bucket != nil)

	node.Next = bucketTree.Bucket.Nodes
	bucketTree.Bucket.Nodes = node
	bucketTree.Bucket.Count++
	bucketTree.Bucket.LastRefreshed = time.Now()

	// are we over capacity now? then split the bucket
	if bucketTree.Bucket.Count > maxNodesPerBucket {
		bucketTree.LeftChild = &BucketTree{
			Level:  bucketTree.Level + 1,
			Bucket: &Bucket{},
		}
		bucketTree.RightChild = &BucketTree{
			Level:  bucketTree.Level + 1,
			Bucket: &Bucket{},
		}
		idIndex := bucketTree.Level / 8
		bitMask := byte(1 << (7 - bucketTree.Level%8))
		var next *Node
		for cur := bucketTree.Bucket.Nodes; cur != nil; cur = next {
			// cur.Next will be changed in the recursive call, save it now
			next = cur.Next
			if cur.ID[idIndex]&bitMask > 0 {
				bucketTree.RightChild.addNode(cur)
			} else {
				bucketTree.LeftChild.addNode(cur)
			}
		}
		bucketTree.Bucket = nil
	}
}

// makeRoom removes an unknown (non-Good) node from the bucket if there is one
func (bucket *Bucket) makeRoom() bool {
	var last, cur *Node
	for cur = bucket.Nodes; cur != nil && cur.isGood(); cur = cur.Next {
		last = cur
		cur = cur.Next
	}
	if cur != nil {
		// found a non-Good node
		if last != nil {
			last.Next = cur.Next
		} else {
			// it's the first one, we need to update our pointer to the beginning of the linked list
			bucket.Nodes = cur.Next
		}
		bucket.Count--
		return true
	}
	return false
}

// isGood returns true if the Node is "good", i.e. has been active in the last activePeriod (usually 15 minutes).
func (node *Node) isGood() bool {
	return node.LastActive.Add(activePeriod).After(time.Now())
}

// FindNode queries the node for other nodes that are close to the given infohash.
func (node *Node) FindNode(ourID, infohash []byte) (*Node, error) {
	query := NewKRPCFindNodeQuery(ourID, infohash)
	response := KRPCFindNodeResponse{}
	err := Request(node, query, &response)
	if err != nil {
		return nil, err
	}
	count, node, err := response.toNodes()
	if err != nil {
		return nil, err
	}
	log.Printf("Got %d nodes in response", count)
	return node, nil
}

// prefixMatch compares the first bitCount bits of the two byte array slices;
// returns true if they match, false if they don't.
func prefixMatch(ID1, ID2 []byte, bitCount int) bool {
	bytesToMatch := bitCount / 8
	bitsToMatch := bitCount % 8
	if bytesToMatch > 0 && bytes.Compare(ID1[:bytesToMatch], ID2[:bytesToMatch]) != 0 {
		return false
	}

	if bitsToMatch > 0 {
		// we right-shift as many times as there are bits we don't want to match on,
		// as we match from the most significant bit down. So if we want to match the
		// 5 most significant bits then we can just right shift three times, then compare
		// the two bytes.
		byte1 := ID1[bytesToMatch] >> (8 - bitsToMatch)
		byte2 := ID2[bytesToMatch] >> (8 - bitsToMatch)
		return byte1 == byte2
	}

	// bytes matched or no bytes to match, plus no bits to match
	return true
}
