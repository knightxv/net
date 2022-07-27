package kaddht

import (
	"fmt"
	"strings"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/util"
)

type groupTable struct {
	Self       *Node
	groupNodes *util.SafeMap // []*Bucket
}

func newGroupTable(self *Node) *groupTable {
	return &groupTable{
		Self:       self,
		groupNodes: util.NewSafeMap(),
	}
}

func (t *groupTable) getBucket(groupId int, createIfNotExists bool) *Bucket {
	bucket, found := t.groupNodes.Get(groupId)
	if !found {
		if createIfNotExists {
			bucket = NewBucket()
			t.groupNodes.Set(groupId, bucket)
		} else {
			return nil
		}
	}
	return bucket.(*Bucket)
}

func (t *groupTable) getBucketById(id deviceid.DeviceId, createIfNotExists bool) *Bucket {
	groupId := id.GroupIdValue()
	return t.getBucket(groupId, createIfNotExists)
}

func (t *groupTable) getNode(id deviceid.DeviceId) *Node {
	bucket := t.getBucketById(id, false)
	if bucket != nil {
		node, _ := bucket.nodes.Get(id.String())
		if node != nil {
			return node.(*NodeWithTimestamp).Node
		}
	}

	return nil
}

func (t *groupTable) getRandomIDFromBucket(groupId []byte) deviceid.DeviceId {
	return deviceid.NewDeviceId(false, groupId, nil)
}

func (t *groupTable) clearExpiredNodes(expiredDuration time.Duration) {
	for key, _bucket := range t.groupNodes.Items() {
		bucket := _bucket.(*Bucket)
		bucket.ClearExpiredNodes(expiredDuration)
		if bucket.Size() == 0 {
			t.groupNodes.Delete(key)
		}
	}
}

func (t *groupTable) doesNodeExist(id deviceid.DeviceId) bool {
	bucket := t.getBucketById(id, false)
	if bucket != nil {
		return bucket.nodes.Has(id.String())
	} else {
		return false
	}
}

func (t *groupTable) getClosestContacts(num int, target deviceid.DeviceId, filter func(*Node) bool) *shortList {
	bucket := t.getBucketById(target, false)
	sl := newShortList(target)

	if bucket == nil || bucket.Size() == 0 {
		return sl
	}

	leftToAdd := num
	for _, _n := range bucket.nodes.Items() {
		n := _n.(*NodeWithTimestamp)
		if filter == nil || filter(n.Node) {
			sl.AppendUnique([]*Node{n.Node})
			leftToAdd--
			if leftToAdd == 0 {
				break
			}
		}
	}

	return sl.Sort()
}

func (t *groupTable) totalNodes() int {
	total := 0
	for _, _bucket := range t.groupNodes.Items() {
		bucket := _bucket.(*Bucket)
		total += bucket.Size()
	}
	return total
}

func (gt *groupTable) IterateBuckets(filter func(*Bucket) bool, callback func(*Bucket) (next bool)) {
	for _, _bucket := range gt.groupNodes.Items() {
		bucket := _bucket.(*Bucket)
		if filter == nil || filter(bucket) {
			next := callback(bucket)
			if !next {
				break
			}
		}
	}
}

func (gt *groupTable) IterateNodes(filter func(*Node) bool, callback func(int, int, *Node) (next bool)) {
	var idx int
	gt.IterateBuckets(nil, func(b *Bucket) (next bool) {
		idx++
		nodeNum := b.Size()
		b.IterateNode(filter, func(n *Node) (next bool) {
			return callback(idx, nodeNum, n)
		})
		return true
	})
}
func (gt *groupTable) DumpNodes(stringBuilder *strings.Builder) {
	int2ip := func(i int) string {
		j := uint32(i)
		return fmt.Sprintf("%d.%d.%d.%d", j>>24, (j>>16)&0xff, (j>>8)&0xff, j&0xff)
	}

	stringBuilder.WriteString("grouptable:\n")
	for i, _bucket := range gt.groupNodes.Items() {
		bucket := _bucket.(*Bucket)
		if bucket.Size() > 0 {
			stringBuilder.WriteString(fmt.Sprintf("\tbucket %s(total %d):\n", int2ip(i.(int)), bucket.Size()))
			for _, _node := range bucket.nodes.Items() {
				node := _node.(*NodeWithTimestamp).Node
				stringBuilder.WriteString(fmt.Sprintf("\t\tnode %s:\n", node.Id))
			}
		}
	}
}
