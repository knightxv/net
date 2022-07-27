package kaddht

import (
	"math/big"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
)

type Node struct {
	*device_info.DeviceInfo
}

type NodeWithTimestamp struct {
	*Node
	Timestamp time.Time
}

func NewNode(networkNode *device_info.DeviceInfo) *Node {
	return &Node{
		DeviceInfo: networkNode,
	}
}

func NewNodeWithTimestamp(n *Node) *NodeWithTimestamp {
	return &NodeWithTimestamp{
		Node:      n,
		Timestamp: time.Now(),
	}
}

func (n *NodeWithTimestamp) UpdateTimestamp() {
	n.Timestamp = time.Now()
}

func (n *Node) GetDeviceInfo() *device_info.DeviceInfo {
	return n.DeviceInfo
}

func (n *Node) Equal(other *Node, allowNilID bool) bool {
	if n == nil || other == nil {
		return false
	}
	if !allowNilID {
		return n.Id.SameNodeId(other.Id)
	}
	return true
}

func (n *Node) Distance(target deviceid.DeviceId) *big.Int {
	return n.Id.GetDistance(target)
}

func (n *Node) IsCloser(target deviceid.DeviceId, other *Node) bool {
	return n.Distance(target).Cmp(other.Distance(target)) == -1
}
