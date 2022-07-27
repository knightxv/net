package kaddht

import (
	"sort"

	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
)

type nodeItem struct {
	node  *Node
	index int
}

type shortList struct {
	// Nodes are a list of nodes to be compared
	nodes   []*Node
	nodeMap map[string]*nodeItem
	// Comparator is the ID to compare to
	comparator deviceid.DeviceId
}

func newShortList(comparator deviceid.DeviceId) *shortList {
	return &shortList{
		comparator: comparator,
		nodeMap:    make(map[string]*nodeItem),
		nodes:      make([]*Node, 0),
	}
}

func (n *shortList) AppendUnique(nodes []*Node) {
	for _, vv := range nodes {
		_, found := n.nodeMap[vv.Id.String()]
		if !found {
			n.nodes = append(n.nodes, vv)
			n.nodeMap[vv.Id.String()] = &nodeItem{node: vv, index: len(n.nodes) - 1}
		}
	}
}

func (n *shortList) First() *Node {
	return n.At(0)
}

func (n *shortList) At(i int) *Node {
	if i < 0 || i >= len(n.nodes) {
		return nil
	}

	return n.nodes[i]
}

func (n *shortList) Nodes() []*Node {
	return n.nodes
}

func (n *shortList) Len() int {
	return len(n.nodes)
}

func (n *shortList) Swap(i, j int) {
	n.nodes[i], n.nodes[j] = n.nodes[j], n.nodes[i]
	n.nodeMap[n.nodes[i].Id.String()].index = j
	n.nodeMap[n.nodes[j].Id.String()].index = i
}

func (n *shortList) Less(i, j int) bool {
	return n.nodes[i].IsCloser(n.comparator, n.nodes[j])
}

func (n *shortList) Sort() *shortList {
	sort.Sort(n)
	return n
}

func (n *shortList) RemoveNode(node *Node) {
	key := node.Id.String()
	item, found := n.nodeMap[key]
	if found {
		n.nodes = append(n.nodes[:item.index], n.nodes[item.index+1:]...)
		delete(n.nodeMap, key)
	}
}
