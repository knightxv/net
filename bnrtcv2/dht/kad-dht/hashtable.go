package kaddht

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/util"
)

type hashTable struct {
	Self         *Node
	RoutingTable *util.SafeSlice // []*Bucket
}

func newHashTable(self *Node) *hashTable {
	ht := &hashTable{
		Self:         self,
		RoutingTable: util.NewSafeSlice(DeviceIDBits),
	}

	for i := 0; i < DeviceIDBits; i++ {
		ht.RoutingTable.Set(i, NewBucket())
	}

	return ht
}

func (ht *hashTable) getBucket(bucketIdx int) *Bucket {
	bucket, _ := ht.RoutingTable.Get(bucketIdx)
	return bucket.(*Bucket)
}

func (ht *hashTable) IterateBuckets(filter func(bucket *Bucket) bool, callback func(idx int, bucket *Bucket) (next bool)) {
	for idx, _bucket := range ht.RoutingTable.Items() {
		bucket := _bucket.(*Bucket)
		if filter == nil || filter(bucket) {
			next := callback(idx, bucket)
			if !next {
				break
			}
		}
	}
}

func (ht *hashTable) IterateNodes(filter func(*Node) bool, callback func(int, int, *Node) (next bool)) {
	ht.IterateBuckets(nil, func(idx int, b *Bucket) (next bool) {
		nodeNum := b.Size()
		b.IterateNode(filter, func(n *Node) (next bool) {
			return callback(idx, nodeNum, n)
		})
		return true
	})
}

func (ht *hashTable) getNode(id deviceid.DeviceId) *Node {
	index := ht.calculateBucketIdx(id)
	bucket := ht.getBucket(index)
	node, _ := bucket.nodes.Get(id.String())
	if node != nil {
		return node.(*NodeWithTimestamp).Node
	}

	return nil
}

// func (ht *hashTable) removeNode(id deviceid.DeviceId) {
// 	index := ht.calculateBucketIdx(id)
// 	bucket := ht.getBucket(index)
// 	bucket.nodes.Delete(id.String())
// }

func (ht *hashTable) calculateBucketIdx(id deviceid.DeviceId) int {
	selfTopId := ht.Self.Id.ToTopNetwork() // use top network id for hashing
	diffBit := selfTopId.HighestDifferBit(id)
	return DeviceIDBits - diffBit - 1
}

func (ht *hashTable) resetRefreshTimeForBucket(bucketIdx int) {
	ht.getBucket(bucketIdx).ResetRefreshTime()
}

func (ht *hashTable) clearExpiredNodes(expiredDuration time.Duration) {
	for _, _bucket := range ht.RoutingTable.Items() {
		_bucket.(*Bucket).ClearExpiredNodes(expiredDuration)
	}
}

func (ht *hashTable) doesNodeExistInBucket(bucketIdx int, id deviceid.DeviceId) bool {
	return ht.getBucket(bucketIdx).nodes.Has(id.String())
}

func (ht *hashTable) getClosestContacts(num int, target deviceid.DeviceId, filter func(*Node) bool) *shortList {
	index := ht.calculateBucketIdx(target)
	indexList := []int{index}
	i := index - 1
	j := index + 1
	for len(indexList) < DeviceIDBits {
		if j < DeviceIDBits {
			indexList = append(indexList, j)
		}
		if i >= 0 {
			indexList = append(indexList, i)
		}
		i--
		j++
	}

	sl := newShortList(target)
	leftToAdd := num
	for leftToAdd > 0 && len(indexList) > 0 {
		index, indexList = indexList[0], indexList[1:]
		bucket := ht.getBucket(index)
		for _, _n := range bucket.nodes.Items() {
			n := _n.(*NodeWithTimestamp).Node
			if filter == nil || filter(n) {
				sl.AppendUnique([]*Node{n})
				leftToAdd--
				if leftToAdd == 0 {
					break
				}
			}
		}
	}

	return sl.Sort()
}

func (ht *hashTable) getAllNodesInBucketCloserThan(bucketIdx int, id deviceid.DeviceId) []*Node {
	bucket := ht.getBucket(bucketIdx)
	var nodes []*Node
	for _, _n := range bucket.nodes.Items() {
		n := _n.(*NodeWithTimestamp).Node
		d1 := id.GetDistance(ht.Self.Id)
		d2 := id.GetDistance(n.Id)

		result := d1.Sub(d1, d2)
		if result.Sign() > -1 {
			nodes = append(nodes, n)
		}
	}

	return nodes
}

func (ht *hashTable) getTotalNodesInBucket(bucketIdx int) int {
	bucket := ht.getBucket(bucketIdx)
	return bucket.Size()
}

func (ht *hashTable) getRandomIDFromBucket(bucketIdx int) deviceid.DeviceId {
	diffStartIdx := deviceid.DeviceIdCalculateLength*8 - bucketIdx - 1
	byteIndex := diffStartIdx / 8
	var id []byte
	id = append(id, ht.Self.Id.ToTopNetwork()[:deviceid.DeviceIdCalculateOffset]...)
	for i := 0; i < byteIndex; i++ {
		id = append(id, ht.Self.Id[i+deviceid.DeviceIdCalculateOffset])
	}
	differingBitStart := diffStartIdx % 8

	var firstByte byte
	for i := 0; i < 8; i++ {
		// Set the value of the bit to be the same as the ID
		// up to the differing bit. Then begin randomizing
		var bit bool
		if i < differingBitStart {
			bit = util.HasBit(ht.Self.Id[byteIndex+deviceid.DeviceIdCalculateOffset], uint(i))
		} else {
			bit = rand.Intn(2) == 1
		}

		if bit {
			firstByte += byte(math.Pow(2, float64(7-i)))
		}
	}

	id = append(id, firstByte)

	for i := byteIndex + deviceid.DeviceIdCalculateOffset + 1; i < deviceid.DeviceIdCalculateOffset+deviceid.DeviceIdCalculateLength; i++ {
		randomByte := byte(rand.Intn(256))
		id = append(id, randomByte)
	}
	id = append(id, ht.Self.Id[deviceid.NodeIdOffset:]...)
	return deviceid.DeviceId(id)
}

func (ht *hashTable) totalNodes() int {
	total := 0
	for _, _bucket := range ht.RoutingTable.Items() {
		bucket := _bucket.(*Bucket)
		total += bucket.Size()
	}
	return total
}

func (ht *hashTable) DumpNodes(stringBuilder *strings.Builder) {
	stringBuilder.WriteString("hashtable:\n")
	for i, _bucket := range ht.RoutingTable.Items() {
		bucket := _bucket.(*Bucket)
		if bucket.Size() > 0 {
			stringBuilder.WriteString(fmt.Sprintf("\tbucket %d(total %d):\n", i, bucket.Size()))
			for _, _node := range bucket.nodes.Items() {
				node := _node.(*NodeWithTimestamp).Node
				stringBuilder.WriteString(fmt.Sprintf("\t\tnode %s:\n", node.Id))
			}
		}
	}
}
