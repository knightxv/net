package kaddht

import (
	"time"

	"github.com/guabee/bnrtc/util"
)

type Bucket struct {
	nodes       *util.SafeLinkMap
	refreshTime time.Time
}

func NewBucket() *Bucket {
	return &Bucket{
		nodes:       util.NewSafeLinkMap(),
		refreshTime: time.Time{},
	}
}

func (bucket *Bucket) ResetRefreshTime() {
	bucket.refreshTime = time.Now()
}

func (bucket *Bucket) GetRefreshTime() time.Time {
	return bucket.refreshTime
}

func (bucket *Bucket) Size() int {
	return bucket.nodes.Size()
}

func (bucket *Bucket) FirstNode() *Node {
	_, n, found := bucket.nodes.Front()
	if !found {
		return nil
	}
	return n.(*NodeWithTimestamp).Node
}

func (bucket *Bucket) RemoveNode(n *Node) {
	bucket.nodes.Delete(n.Id.String())
}

func (bucket *Bucket) AddNode(n *Node) {
	bucket.nodes.Set(n.Id.String(), NewNodeWithTimestamp(n))
	bucket.ResetRefreshTime()
}

func (bucket *Bucket) ClearExpiredNodes(expiredDuration time.Duration) {
	for _, _node := range bucket.nodes.Items() {
		node := _node.(*NodeWithTimestamp)
		if time.Since(node.Timestamp) > expiredDuration {
			bucket.nodes.Delete(node.Id.String())
			break
		}
	}
}

func (bucket *Bucket) IterateNode(filter func(*Node) bool, callback func(*Node) (next bool)) {
	for _, _node := range bucket.nodes.Items() {
		node := _node.(*NodeWithTimestamp).Node
		if filter == nil || filter(node) {
			next := callback(node)
			if !next {
				break
			}
		}
	}
}
