package dht

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// compare two BucketTrees by value (mostly - for BucketTree.Bucket.Nodes we compare the references)
func compare(t *testing.T, result, expected *BucketTree) bool {
	if result == nil && expected == nil {
		return true
	}
	// check BucketTree fields
	if result.Level != expected.Level {
		t.Errorf("expected/actual: Level %v/%v", expected.Level, result.Level)
		return false
	}
	if result.Bucket != nil || expected.Bucket != nil {
		if result.Bucket.Count != expected.Bucket.Count || *result.Bucket.Nodes != *expected.Bucket.Nodes {
			t.Errorf("expected Bucket: %v, actual Bucket: %v", *expected.Bucket, *result.Bucket)
			return false
		}
		// make sure LastRefreshed was set properly, it won't be set on bt2
		if result.Bucket.LastRefreshed.Add(2 * time.Second).Before(time.Now()) {
			t.Errorf("Last refreshed: %s, Now: %s", result.Bucket.LastRefreshed, time.Now())
			return false
		}
	}
	return compare(t, result.LeftChild, expected.LeftChild) && compare(t, result.RightChild, expected.RightChild)
}

func TestAddNode(t *testing.T) {
	testNode := &Node{
		ID: &[20]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	existingNode := &Node{
		ID: &[20]byte{8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	expectedNode := *testNode
	expectedNode.Next = existingNode
	input := BucketTree{
		Level: 0,
		Bucket: &Bucket{
			Nodes: existingNode,
			Count: 1,
		},
	}
	expected := BucketTree{
		Level: 0,
		Bucket: &Bucket{
			Nodes: &expectedNode,
			Count: 2,
		},
	}

	input.addNode(testNode)
	// the data is modified in-place, so the input is the result
	assert.True(t, compare(t, &input, &expected))
}

func TestAddNodeBucketSplit(t *testing.T) {
	testNode := &Node{
		ID: &[20]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	existingNode := &Node{
		ID: &[20]byte{8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	input := BucketTree{
		Level: 4,
		Bucket: &Bucket{
			Nodes: existingNode,
			Count: 8,
		},
	}
	expected := BucketTree{
		Level: 4,
		LeftChild: &BucketTree{
			Level: 5,
			Bucket: &Bucket{
				Nodes: testNode,
				Count: 1,
			},
		},
		RightChild: &BucketTree{
			Level: 5,
			Bucket: &Bucket{
				// we've set exactly one bit in existingNode, which should match the level
				// and cause it to be put into RightChild
				Nodes: existingNode,
				Count: 1,
			},
		},
	}

	input.addNode(testNode)
	// the data is modified in-place, so the input is the result
	assert.True(t, compare(t, &input, &expected))
}

func TestInsertNode(t *testing.T) {
	node := &Node{
		ID: &[20]byte{128, 0, 0, 0, 0, 0, 0, 0, 42, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	dht := &DHT{
		NodeID: &[20]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		BucketTree: &BucketTree{
			Level: 0,
			LeftChild: &BucketTree{
				Level:  1,
				Bucket: &Bucket{},
			},
			RightChild: &BucketTree{
				Level: 1,
				LeftChild: &BucketTree{
					Level:  2,
					Bucket: &Bucket{},
				},
				RightChild: &BucketTree{
					Level:  2,
					Bucket: &Bucket{},
				},
			},
		},
	}

	dht.InsertNode(node)
	assert.Equal(t, node, dht.BucketTree.RightChild.LeftChild.Bucket.Nodes)
}
