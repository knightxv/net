package kaddht

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/guabee/bnrtc/bnrtcv2/config"
	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/dht/idht"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/util"
	"github.com/robfig/cron/v3"
)

type iterateType uint8

const (
	iterateStore iterateType = iota
	iterateFindNode
	iterateFindValue
)

const (
	// a small number representing the degree of parallelism in network calls
	ParallelDegree = 3

	// the size in bits of the keys used to identify nodes and store and
	// retrieve data; in basic Kademlia this is 160, the length of a SHA1
	DeviceIDBits = deviceid.DeviceIdCalculateLength * 8

	// the maximum number of contacts stored in a bucket
	MaxNodeNumPerBucket = 20
)

const (
	ReplicateTimeMin          = 10 * time.Second
	ReplicateTimeDefault      = 3600 * time.Second
	RefreshTimeDefault        = 3600 * time.Second
	RePublishTimeDefault      = 24 * time.Hour
	ExpireTimeMax             = RePublishTimeDefault + 10*time.Second
	ExpireTimeMin             = 10 * time.Second
	PingTimeMax               = 1 * time.Second
	MessageTimeoutDefault     = 10 * time.Second
	CheckBootstrapInterval    = 30 * time.Second
	UpdateHashtableInterval   = 30 * time.Second
	UPdateClosestNodeInterval = 30 * time.Second
)

type DHT struct {
	Self                  *Node
	IsTopNetwork          bool
	DeviceInfo            *device_info.DeviceInfo
	ht                    *hashTable
	gt                    *groupTable
	options               *Options
	networking            networking
	store                 Store
	closestTopNetworkNode *Node
	crontab               *cron.Cron
	stopChan              chan struct{}
	wg                    sync.WaitGroup
	bootstrapTask         *util.SingleTonTask
	updateClosetNodeTask  *util.SingleTonTask
	logger                *log.Entry
	singleflight          singleflight.Group
}

// Options contains configuration options for the local node
type Options struct {
	DeviceInfo *device_info.DeviceInfo

	// Peers []*transport.TransportAddress
	ID deviceid.DeviceId

	// Specifies the the host of the STUN server. If left empty will use the
	// default specified in go-stun.

	// The nodes being used to bootstrap the network. Without a bootstrap
	// node there is no way to connect to the network.
	BootstrapNodes []*Node

	// The time after which a key/value pair expires;
	// this is a time-to-live (TTL) from the original publication date
	TExpire time.Duration

	// Seconds after which an otherwise unaccessed bucket must be refreshed
	TRefresh time.Duration

	// The interval between Kademlia replication events, when a node is
	// required to publish its entire database
	TReplicate time.Duration

	// The time after which the original publisher must
	// republish a key/value pair. Currently not implemented.
	TRepublish time.Duration

	// The maximum time to wait for a response from a node before discarding
	// it from the bucket
	TPingMax time.Duration

	// The maximum time to wait for a response to any message
	TMsgTimeout time.Duration

	TCheckBootstrap     time.Duration
	TUpdateHashtable    time.Duration
	TUPdateClosestNode  time.Duration
	ParallelDegree      int
	MaxNodeNumPerBucket int
	AutoRun             bool
	PubSub              *util.PubSub

	DHTMemoryStoreOptions *config.DHTMemoryStoreOptions
}

// NewDHT initializes a new DHT node. A store and options struct must be provided.
func NewDHT(options *Options, transportManager idht.ITransportManager) (*DHT, error) {
	if options == nil || !options.DeviceInfo.IsValid() || transportManager == nil {
		return nil, fmt.Errorf("invalid options")
	}
	self := NewNode(options.DeviceInfo)
	dht := &DHT{
		options:              options,
		Self:                 self,
		IsTopNetwork:         self.Id.IsTopNetwork(),
		DeviceInfo:           options.DeviceInfo,
		store:                newMemoryStore(options.DHTMemoryStoreOptions),
		ht:                   newHashTable(self),
		gt:                   newGroupTable(self),
		crontab:              cron.New(cron.WithSeconds()),
		stopChan:             make(chan struct{}),
		bootstrapTask:        util.NewSingleTonTask(),
		updateClosetNodeTask: util.NewSingleTonTask(),
		logger:               log.NewLoggerEntry("dht").WithField("id", self.Id.String()),
	}
	dht.networking = newRealNetworking(transportManager, self, dht.logger)
	netMsgInit()

	if options.TExpire < ExpireTimeMin || options.TExpire > ExpireTimeMax {
		dht.options.TExpire = ExpireTimeMax
	}
	if options.TReplicate <= 0 {
		options.TReplicate = ReplicateTimeDefault
	}
	if options.TReplicate <= ReplicateTimeMin {
		dht.options.TReplicate = ReplicateTimeMin
	}

	if options.TRefresh <= 0 {
		options.TRefresh = RefreshTimeDefault
	}

	if options.TRepublish <= 0 {
		options.TRepublish = RePublishTimeDefault
	}

	if options.TPingMax <= 0 {
		options.TPingMax = PingTimeMax
	}

	if options.TMsgTimeout <= 0 {
		options.TMsgTimeout = MessageTimeoutDefault
	}

	if options.TCheckBootstrap <= 0 {
		options.TCheckBootstrap = CheckBootstrapInterval
	}

	if options.TUpdateHashtable <= 0 {
		options.TUpdateHashtable = UpdateHashtableInterval
	}

	if options.TUPdateClosestNode <= 0 {
		options.TUPdateClosestNode = UPdateClosestNodeInterval
	}

	if options.ParallelDegree <= 0 {
		options.ParallelDegree = ParallelDegree
	}

	if options.MaxNodeNumPerBucket <= 0 {
		options.MaxNodeNumPerBucket = MaxNodeNumPerBucket
	}

	if options.PubSub == nil {
		options.PubSub = util.DefaultPubSub()
	}

	if options.AutoRun {
		dht.Run()
	}
	dht.logger.Info("DHT node started")
	dht.options.PubSub.Publish(rpc.DHTStatusChangeTopic, idht.DHTStatusReady)
	return dht, nil
}

func (dht *DHT) getReplicationTime(duration time.Duration) time.Time {
	if duration < ReplicateTimeMin {
		duration = dht.options.TReplicate
	}

	return time.Now().Add(duration)

}

func (dht *DHT) getExpirationTime(key deviceid.DataKey, duration time.Duration) time.Time {
	// if user configured a custom expiration time, use it
	if duration >= ExpireTimeMin && duration <= ExpireTimeMax {
		return time.Now().Add(duration)
	}

	// otherwise, use the default expiration time, and avoid over-cached
	duration = dht.options.TExpire
	keyId := key.ToDeviceId()
	bucket := dht.ht.calculateBucketIdx(keyId)
	var total int
	for i := 0; i < bucket; i++ {
		total += dht.ht.getTotalNodesInBucket(i)
	}
	closer := dht.ht.getAllNodesInBucketCloserThan(bucket, keyId)
	score := total + len(closer)

	if score < dht.options.MaxNodeNumPerBucket {
		return time.Now().Add(duration)
	}

	seconds := duration.Nanoseconds() / int64(math.Exp(float64(score/dht.options.MaxNodeNumPerBucket)))
	dur := time.Duration(seconds)
	if dur < ExpireTimeMin {
		dur = ExpireTimeMin
	}

	return time.Now().Add(dur)
}

func (dht *DHT) getClosestContacts(num int, target deviceid.DeviceId, filter func(*Node) bool) *shortList {
	if target.IsTopNetwork() {
		return dht.ht.getClosestContacts(num, target, filter)
	} else {
		return dht.gt.getClosestContacts(num, target, filter)
	}
}
func (dht *DHT) GetClosestContacts(num int, target deviceid.DeviceId, filter func(*Node) bool) *shortList {
	return dht.getClosestContacts(num, target, filter)
}

func (dht *DHT) sendRequestMessage(t messageType, receiver *Node, options *messageRequestOptions) (*networkMessageResponse, error) {
	var queryData interface{} = nil
	if options == nil {
		options = &messageRequestOptions{}
	}
	noNeedResponse := false
	switch t {
	case messageTypePing:
		// use empty queryData
	case messageTypeFindNode:
		queryData = &queryDataFindNode{
			Target: options.Target,
		}
	case messageTypeStore:
		queryData = &queryDataStore{
			Key:     options.Target,
			Data:    options.Data,
			Options: options.Args.(*idht.DHTStoreOptions),
		}
	case messageTypeFindValue:
		queryData = &queryDataFindValue{
			Target:  options.Target,
			NoCache: options.Args.(bool),
		}
	case messageTypeJoinGroup:
		// use empty queryData
	case messageTypeLeaveGroup:
		noNeedResponse = true
		// use empty queryData
	default:
		panic("Unknown iterate type")
	}

	query := newMessage(t, dht.Self, receiver, &messageOptionData{Data: queryData, Iterative: options.Iterative})
	timeout := options.Timeout
	if noNeedResponse {
		timeout = 0
	} else if timeout <= 0 {
		if options.Iterative {
			timeout = dht.options.TMsgTimeout * 2 // double the timeout for iterative queries
		} else {
			timeout = dht.options.TMsgTimeout
		}
	}
	return dht.networking.sendMessage(query, &messageResponseOptions{timeout: timeout})
}

func (dht *DHT) sendResponseMessage(t messageType, receiver *Node, msgId int64, data interface{}) {
	response := newMessage(t, dht.Self, receiver, &messageOptionData{Data: data, ID: msgId, IsResponse: true})
	_, _ = dht.networking.sendMessage(response, nil)
}

// Iterate does an iterative search through the network. This can be done
// for multiple reasons. These reasons include:
//     iterativeStore - Used to store new information in the network.
//     iterativeFindNode - Used to bootstrap the network.
//     iterativeFindValue - Used to find a value among the network given a key.
func (dht *DHT) doIterate(t iterateType, sl *shortList, target []byte, data []byte, args interface{}) (storeData *StoreData, closest []*Node, err error) {
	// We keep track of nodes contacted so far. We don't contact the same node
	// twice.
	var contacted = make(map[string]bool)
	queryRest := false
	noCache := false

	// We keep a reference to the closestNode. If after performing a search
	// we do not find a closer node, we stop searching.
	if sl.Len() == 0 {
		return nil, nil, nil
	}

	closestNode := sl.First()
	var closestStoreData *StoreData = nil
	var closestStoreDataNode *Node = nil

	if deviceid.DeviceId(target).IsTopNetwork() {
		bucketIdx := dht.ht.calculateBucketIdx(target)
		dht.ht.resetRefreshTimeForBucket(bucketIdx)
	}
	if t == iterateFindValue {
		noCache = args.(bool)
	}

	removeFromShortlist := []*Node{}
	for {
		numExpectedResponses := 0
		nodes := sl.Nodes()
		reqChan := make([]interface{}, 0)
		// Next we send messages to the first (closest) alpha nodes in the
		// shortlist and wait for a response
		for _, node := range nodes {
			// Contact only alpha nodes
			if numExpectedResponses >= dht.options.ParallelDegree && !queryRest {
				break
			}

			// Don't contact nodes already contacted
			if contacted[node.Id.String()] {
				continue
			}

			contacted[node.Id.String()] = true
			res := (*networkMessageResponse)(nil)
			switch t {
			case iterateFindNode:
				res, err = dht.sendRequestMessage(messageTypeFindNode, node, &messageRequestOptions{Target: target})
			case iterateStore: // iterateStore use findNode to get the closest nodes, and then send StoreMessage to the closest nodes
				res, err = dht.sendRequestMessage(messageTypeFindNode, node, &messageRequestOptions{Target: target})
			case iterateFindValue:
				res, err = dht.sendRequestMessage(messageTypeFindValue, node, &messageRequestOptions{Target: target, Args: args})
			default:
				panic("Unknown iterate type")
			}
			if err != nil {
				// Node was unreachable for some reason. We will have to remove
				// it from the shortlist, but we will keep it in our routing
				// table in hopes that it might come back online in the future.
				removeFromShortlist = append(removeFromShortlist, node)
				continue
			}
			resChan := res.ResultChannel()
			if resChan != nil {
				reqChan = append(reqChan, res.ch)
				numExpectedResponses++
			}
		}

		if numExpectedResponses > 0 {
			ctx, cancel := context.WithCancel(context.Background())
			resultChan := make(chan interface{}, len(reqChan))
			go util.SelectN(ctx, reqChan, resultChan)
			for _result := range resultChan {
				if _result != nil {
					result := _result.(*message)
					if result == nil {
						continue
					}
					dht.AddNode(result.Sender, false)
					if result.Error != nil {
						removeFromShortlist = append(removeFromShortlist, result.Sender)
						continue
					}
					switch t {
					case iterateFindNode:
						responseData := result.Data.(*responseDataFindNode)
						sl.AppendUnique(responseData.Closest)
					case iterateStore: // iterateStore use findNode to get the closest nodes, and then send StoreMessage to the closest nodes
						responseData := result.Data.(*responseDataFindNode)
						sl.AppendUnique(responseData.Closest)
					case iterateFindValue:
						responseData := result.Data.(*responseDataFindValue)
						sl.AppendUnique(responseData.Closest)
						if responseData.Data != nil {
							if !noCache {
								// when no cache is set, we return the data found from any nodes
								dht.logger.Debugf("Found value for %s in network, node %s", target, result.Sender.Id)
								cancel()
								return responseData.Data, []*Node{result.Sender}, nil
							} else {
								if closestStoreData == nil {
									closestStoreData = responseData.Data
									closestStoreDataNode = result.Sender
								} else if result.Sender.IsCloser(target, closestStoreDataNode) {
									closestStoreData = responseData.Data
									closestStoreDataNode = result.Sender
								}
							}
						}
					}
				}
			}
			cancel()
		}

		for _, n := range removeFromShortlist {
			sl.RemoveNode(n)
		}

		if sl.Len() == 0 {
			return nil, nil, nil
		}

		sl.Sort()

		// If closestNode is unchanged then we are done
		if sl.First().Id.Equal(closestNode.Id) || queryRest {
			// We are done
			switch t {
			case iterateFindNode:
				if !queryRest {
					queryRest = true
					continue
				}
				return nil, sl.Nodes(), nil
			case iterateFindValue:
				if closestStoreData != nil {
					if closestStoreDataNode != nil && !closestNode.Equal(closestStoreDataNode, false) {
						// if closestNode do not store the data, ask it to store
						_, _ = dht.sendRequestMessage(messageTypeStore, closestNode, &messageRequestOptions{Target: target, Args: closestStoreData.Options})
					}
					return closestStoreData, []*Node{closestStoreDataNode}, nil
				}

				return nil, sl.Nodes(), nil
			case iterateStore:
				for i, n := range sl.Nodes() {
					if i >= dht.options.ParallelDegree {
						return nil, nil, nil
					}
					_, _ = dht.sendRequestMessage(messageTypeStore, n, &messageRequestOptions{Target: target, Data: data, Args: args})
				}
				return nil, nil, nil
			}
		} else {
			closestNode = sl.First()
		}
	}
}

func (dht *DHT) iterate(t iterateType, target deviceid.DeviceId, data []byte, args interface{}) (storeData *StoreData, closest []*Node, err error) {
	// if we are top network, and target is sub network, then we should find the closest node in the top network
	if dht.IsTopNetwork && !target.IsTopNetwork() {
		cloestNode := dht.Self
		_, nodes, _ := dht.iterate(iterateFindNode, target.ToTopNetwork(), nil, nil)
		if len(nodes) != 0 {
			if !dht.Self.IsCloser(target, nodes[0]) {
				cloestNode = nodes[0]
			}
		}

		// if we are the closest node, then we iterate in our group table
		if cloestNode == dht.Self {
			sl := dht.gt.getClosestContacts(dht.options.ParallelDegree, target, nil)
			return dht.doIterate(t, sl, target, data, args)
		} else {
			// if we are not the closest node, then we ask the closest node to iterate it for us
			res := (*networkMessageResponse)(nil)
			switch t {
			case iterateFindNode:
				res, err = dht.sendRequestMessage(messageTypeFindNode, cloestNode, &messageRequestOptions{Target: target, Iterative: true})
			case iterateStore:
				res, err = dht.sendRequestMessage(messageTypeStore, cloestNode, &messageRequestOptions{Target: target, Data: data, Args: args, Iterative: true})
			case iterateFindValue:
				res, err = dht.sendRequestMessage(messageTypeFindValue, cloestNode, &messageRequestOptions{Target: target, Args: args, Iterative: true})
			default:
				panic("Unknown iterate type")
			}
			if err != nil {
				return nil, nil, err
			}
			if res == nil {
				// not expected response
				return nil, nil, nil
			}

			result := res.WaitResult()
			if result != nil {
				switch t {
				case iterateFindNode:
					responseData := result.Data.(*responseDataFindNode)
					return nil, responseData.Closest, nil
				case iterateFindValue:
					responseData := result.Data.(*responseDataFindValue)
					return responseData.Data, responseData.Closest, nil
				}
			} else {
				return nil, nil, nil
			}
		}
		return nil, nil, nil
	}

	var sl *shortList
	if target.IsTopNetwork() {
		sl = dht.ht.getClosestContacts(dht.options.ParallelDegree, target, nil)
	} else {
		sl = dht.gt.getClosestContacts(dht.options.ParallelDegree, target, nil)
		if !dht.IsTopNetwork { // if we are not top network, then we should add the close topNetwork node to the shortlist
			topNode := dht.closestTopNetworkNode
			if topNode != nil {
				sl.AppendUnique([]*Node{topNode})
				sl.Sort()
			}
		}
	}

	return dht.doIterate(t, sl, target, data, args)
}

func (dht *DHT) CheckAndRemoveNode(bucket *Bucket) bool {
	// If the bucket is full we need to ping the first node to find out
	// if it responds back in a reasonable amount of time. If not -
	// we may remove it
	n := bucket.FirstNode()
	res, err := dht.sendRequestMessage(messageTypePing, n, nil)
	if err != nil {
		bucket.RemoveNode(n)
		return true
	} else {
		result := res.WaitResult()
		if result == nil { // timeout
			bucket.RemoveNode(n)
			return true
		}
	}

	return false

}

// addNode adds a node into the appropriate k bucket
// we store these buckets in big-endian order so we look at the bits
// from right to left in order to find the appropriate bucket
func (dht *DHT) AddNode(node *Node, join bool) {
	if dht.store.CheckOnBlacklist(deviceid.DataKey(node.Id)) {
		return
	}
	if node == nil {
		return
	}

	if dht.Self.Equal(node, false) {
		// ignore ourself
		return
	}

	dht.logger.Debugf("Adding node %s", node.Id)
	if !node.Id.IsTopNetwork() && !dht.gt.doesNodeExist(node.Id) {
		// topNetwork node only add subNetwork node from Join message
		if dht.IsTopNetwork && !join {
			return
		} else if !dht.IsTopNetwork && !dht.Self.Id.InSameGroup(node.Id) { // subNetwork node only add subNetwork node in same group
			return
		}
	}

	oldNodeNum := dht.NumNodes(true)
	nodeExist := false
	var bucket *Bucket
	if !node.Id.IsTopNetwork() {
		nodeExist = dht.gt.doesNodeExist(node.Id)
		bucket = dht.gt.getBucketById(node.Id, true)
	} else {
		index := dht.ht.calculateBucketIdx(node.Id)
		nodeExist = dht.ht.doesNodeExistInBucket(index, node.Id)
		bucket = dht.ht.getBucket(index)
	}

	if !nodeExist && bucket.Size() >= dht.options.MaxNodeNumPerBucket {
		if dht.CheckAndRemoveNode(bucket) {
			bucket.AddNode(node)
		}
	} else {
		bucket.AddNode(node)
	}

	if oldNodeNum == 0 && dht.NumNodes(true) > 0 {
		dht.options.PubSub.Publish(rpc.DHTStatusChangeTopic, idht.DHTStatusConnected)
		dht.doUpdateClosestTopNetwork()
	}
}

func (dht *DHT) removeNode(node *Node) {
	if node == nil {
		return
	}

	dht.logger.Debugf("Removing node %s", node.Id)

	var bucket *Bucket
	if !node.Id.IsTopNetwork() {
		groupId := node.Id.GroupIdValue()
		bucket = dht.gt.getBucket(groupId, false)
	} else {
		index := dht.ht.calculateBucketIdx(node.Id)
		bucket = dht.ht.getBucket(index)
	}

	if bucket != nil {
		bucket.RemoveNode(node)
		if dht.NumNodes(false) == 0 {
			dht.options.PubSub.Publish(rpc.DHTStatusChangeTopic, idht.DHTStatusDisconnected)
		}
	}
}

func (dht *DHT) DumpNodes(stringBuilder *strings.Builder) {
	stringBuilder.WriteString(dht.Self.Id.String())
	stringBuilder.WriteString("\n")
	dht.ht.DumpNodes(stringBuilder)
	dht.gt.DumpNodes(stringBuilder)
}

// NumNodes returns the total number of nodes stored in the local routing table
// ignore nodes in group table
func (dht *DHT) NumNodes(ignoreGroup bool) int {
	if ignoreGroup {
		return dht.ht.totalNodes()
	} else {
		return dht.ht.totalNodes() + dht.gt.totalNodes()
	}
}

func (dht *DHT) IsConnected() bool {
	return dht.NumNodes(false) > 0
}

// Bootstrap attempts to bootstrap the network using the BootstrapNodes provided
// to the Options struct. This will trigger an iterativeFindNode to the provided
// BootstrapNodes.
func (dht *DHT) Bootstrap() {
	bootstrapNodes := make([]*Node, len(dht.options.BootstrapNodes))
	copy(bootstrapNodes, dht.options.BootstrapNodes)
	if len(bootstrapNodes) == 0 {
		return
	}

	results := make([]*networkMessageResponse, 0)
	for _, bn := range bootstrapNodes {
		bn.Id = deviceid.EmptyDeviceId // clear the id
		addr := bn.ToTransportAddress()
		dht.logger.Debugf("Bootstrapping to %s", addr)
		if addr == nil || (addr.IsLocal() && addr.Addr.Port == dht.Self.Port) {
			continue
		}
		res, err := dht.sendRequestMessage(messageTypePing, bn, nil)
		if err != nil {
			continue
		}
		results = append(results, res)
	}

	for _, res := range results {
		result := res.WaitResult()
		// If result is nil, channel was closed
		if result != nil {
			dht.AddNode(result.Sender, false)
		}
	}
}

func (dht *DHT) doBootstrap() {
	dht.bootstrapTask.RunWaitWith(dht.Bootstrap)
}

// Disconnect will trigger a disconnect from the network. All underlying sockets
// will be closed.
func (dht *DHT) Close() error {
	close(dht.stopChan)
	ctx := dht.crontab.Stop()
	<-ctx.Done()
	dht.wg.Wait()
	dht.networking.close()
	return nil
}

// Listen begins listening on the socket for incoming messages
func (dht *DHT) Run() {
	util.GoWithWaitGroup(&dht.wg, dht.listen)
	util.GoWithWaitGroup(&dht.wg, dht.iterateWorker)
	dht.networking.run()
	dht.addTimers()
	dht.crontab.Start()
	util.GoWithWaitGroup(&dht.wg, dht.doBootstrap)
	util.GoWithWaitGroup(&dht.wg, dht.updateHashTableWorker)
}

func (dht *DHT) updateHashTableWorker() {
	timer := time.NewTicker(1 * time.Second)
	idx := 0
	for {
		select {
		case <-dht.stopChan:
			timer.Stop()
			return
		case <-timer.C:
			count := 0
			for {
				curIdx := idx % DeviceIDBits
				bucket := dht.ht.getBucket(curIdx)
				idx += 1
				count += 1
				if time.Since(bucket.GetRefreshTime()) > dht.options.TRefresh {
					id := dht.ht.getRandomIDFromBucket(curIdx)
					_, _, _ = dht.iterate(iterateFindNode, id, nil, nil)
					break
				}
				if count >= DeviceIDBits {
					break
				}
			}
		}
	}
}

func (dht *DHT) iterateWorker() {
	for {
		select {
		case msg := <-dht.networking.getIterateMessageChan():
			if msg == nil {
				return
			}
			filterSender := func(n *Node) bool {
				return msg.Sender.Id.String() != n.Id.String()
			}

			switch msg.Type {
			case messageTypeFindNode:
				data := msg.Data.(*queryDataFindNode)
				dht.AddNode(msg.Sender, false)
				sl := dht.getClosestContacts(dht.options.ParallelDegree, data.Target, filterSender)
				_, nodes, _ := dht.doIterate(iterateFindNode, sl, data.Target, nil, nil)
				dht.sendResponseMessage(messageTypeFindNode, msg.Sender, msg.ID, &responseDataFindNode{Closest: nodes})
			case messageTypeFindValue:
				data := msg.Data.(*queryDataFindValue)
				dht.AddNode(msg.Sender, false)
				sl := dht.getClosestContacts(dht.options.ParallelDegree, data.Target, filterSender)
				storeData, _, _ := dht.doIterate(iterateFindValue, sl, data.Target, nil, data.NoCache)
				dht.sendResponseMessage(messageTypeFindValue, msg.Sender, msg.ID, &responseDataFindValue{Data: storeData})
			case messageTypeStore:
				data := msg.Data.(*queryDataStore)
				dht.AddNode(msg.Sender, false)
				sl := dht.getClosestContacts(dht.options.ParallelDegree, data.Key, filterSender)
				_, _, _ = dht.doIterate(iterateStore, sl, data.Key, data.Data, data.Options)
			default:
				dht.logger.Errorf("ignore iterative message with type %s", msg.Type)
			}
		case <-dht.stopChan:
			return
		}
	}
}

func (dht *DHT) listen() {
	for {
		select {
		case msg := <-dht.networking.getMessageChan():
			if msg == nil {
				return
			}
			filterSender := func(n *Node) bool {
				return msg.Sender.Id.String() != n.Id.String()
			}
			switch msg.Type {
			case messageTypeFindNode:
				data := msg.Data.(*queryDataFindNode)
				dht.AddNode(msg.Sender, false)
				closest := dht.getClosestContacts(dht.options.ParallelDegree, data.Target, filterSender)
				dht.sendResponseMessage(messageTypeFindNode, msg.Sender, msg.ID, &responseDataFindNode{Closest: closest.Nodes()})
			case messageTypeFindValue:
				data := msg.Data.(*queryDataFindValue)
				dht.AddNode(msg.Sender, false)
				storeData, _ := dht.store.Retrieve(data.Target)
				closest := dht.getClosestContacts(dht.options.ParallelDegree, data.Target, filterSender)
				dht.sendResponseMessage(messageTypeFindValue, msg.Sender, msg.ID, &responseDataFindValue{Data: storeData, Closest: closest.Nodes()})
			case messageTypeStore:
				data := msg.Data.(*queryDataStore)
				dht.AddNode(msg.Sender, false)

				key := data.Key
				if key == nil {
					key = deviceid.KeyGen(data.Data)
				}

				options := data.Options
				if options == nil {
					options = &idht.DHTStoreOptions{}
				}
				dht.storeLocal(key, data.Data, false, options)
			case messageTypePing:
				if !dht.Self.Equal(msg.Sender, false) {
					// Ignore ping messages from self, as bootsrap nodes may send this
					dht.AddNode(msg.Sender, false)
					dht.sendResponseMessage(messageTypePing, msg.Sender, msg.ID, nil)
				}
			case messageTypeJoinGroup:
				dht.AddNode(msg.Sender, true)
				dht.sendResponseMessage(messageTypeJoinGroup, msg.Sender, msg.ID, nil)
			case messageTypeLeaveGroup:
				dht.removeNode(msg.Sender)
			}
		case <-dht.stopChan:
			return
		}
	}
}

func (dht *DHT) addTimers() {
	durationToEveryString := func(duration time.Duration) string {
		return "@every " + duration.String()
	}
	addTimer := func(duration time.Duration, f func()) {
		_, err := dht.crontab.AddFunc(durationToEveryString(duration), f)
		if err != nil {
			dht.logger.Errorf("failed to add timer: %s", err)
		}
	}

	// check hashtable bucket
	addTimer(dht.options.TUpdateHashtable, func() {
		if !dht.Self.Id.IsTopNetwork() {
			id := dht.gt.getRandomIDFromBucket(dht.Self.Id.GroupId())
			_, _, _ = dht.iterate(iterateFindNode, id, nil, nil)
		}
	})

	// check store Expiration
	addTimer(1*time.Minute, func() {
		dht.store.ExpireKeys()
	})

	// check hashtable/groutable node Expiration
	addTimer(1*time.Minute, func() {
		dht.gt.clearExpiredNodes(5 * time.Minute)
		dht.ht.clearExpiredNodes(5 * time.Minute)
	})

	// check bootstrap node
	addTimer(dht.options.TCheckBootstrap, func() {
		if dht.NumNodes(true) == 0 {
			dht.doBootstrap()
		}
	})

	// update closest topnetwork node
	addTimer(dht.options.TUPdateClosestNode, func() {
		if dht.NumNodes(true) != 0 {
			dht.doUpdateClosestTopNetwork()
		}
	})
}

func (dht *DHT) pingNode(node *Node) bool {
	res, err := dht.sendRequestMessage(messageTypePing, node, nil)
	if err != nil {
		return false
	}

	result := res.WaitResult()
	return result != nil
}

func (dht *DHT) setClosestTopNetworkNode(node *Node) error {
	oldClosest := dht.closestTopNetworkNode
	// subnetwork node send JoinGroup/LeaveGroup msg to closest topnetwork node
	if !dht.IsTopNetwork {
		// dht.logger.Debugf("updateClosestTopNetwokNode: %s -> %s", oldClosest, node)
		isSameNode := (oldClosest == nil && node == nil) || (oldClosest != nil && node != nil && oldClosest.Id.Equal(node.Id))
		if !isSameNode {
			// leave old
			if oldClosest != nil {
				_, _ = dht.sendRequestMessage(messageTypeLeaveGroup, oldClosest, nil)
			}
			// join new
			res, err := dht.sendRequestMessage(messageTypeJoinGroup, node, nil)
			if err != nil {
				return err
			}
			result := res.WaitResult()
			if result == nil {
				return errors.New("failed to join group")
			}
		}
	}
	dht.closestTopNetworkNode = node
	return nil
}

func (dht *DHT) updateClosestTopNetworkNode() {
	// update closest topnetwork node
	id := dht.Self.Id.ToTopNetwork()
	_, nodes, _ := dht.iterate(iterateFindNode, id, nil, nil)
	for _, node := range nodes {
		isAlive := dht.pingNode(node)
		if isAlive {
			err := dht.setClosestTopNetworkNode(node)
			if err == nil {
				break
			}
		}
	}
}

func (dht *DHT) doUpdateClosestTopNetwork() {
	dht.updateClosetNodeTask.RunWaitWith(dht.updateClosestTopNetworkNode)
}

func (dht *DHT) storeLocal(key deviceid.DataKey, data []byte, isPublisher bool, opts *idht.DHTStoreOptions) {
	if !dht.isValidKey(key) {
		dht.logger.Errorf("invalid key: %s", key)
		return
	}
	expiration := dht.getExpirationTime(key, opts.TExpire)
	replication := dht.getReplicationTime(opts.TReplicate)
	err := dht.store.Store(key, data, replication, expiration, isPublisher, opts)
	if err != nil {
		dht.logger.Debugf("storeLocal failed: %s", err)
	}
}

func (dht *DHT) isValidKey(key deviceid.DataKey) bool {
	return len(key) == deviceid.DeviceIdLength && key.ToDeviceId().IsTopNetwork()
}

func (dht *DHT) Store(key deviceid.DataKey, data []byte, opts *idht.DHTStoreOptions) error {
	dht.logger.Debugf("store key %s", key)
	if data == nil {
		return errors.New("data cannot be nil")
	}

	if key == nil {
		key = deviceid.KeyGen(data)
	} else if !dht.isValidKey(key) {
		return errors.New("invalid key")
	}

	if opts == nil {
		opts = &idht.DHTStoreOptions{}
	}

	dht.storeLocal(key, data, true, opts)
	_, _, err := dht.iterate(iterateStore, key.ToDeviceId(), data, opts)
	if err != nil {
		return err
	}
	return nil
}

func (dht *DHT) Put(key deviceid.DataKey, data []byte, opts *idht.DHTStoreOptions) error {
	return dht.Store(key, data, opts)
}

func (dht *DHT) retrieve(key deviceid.DataKey, noCache bool) (data []byte, exist bool, err error) {
	storeData, nodes, err := dht.iterate(iterateFindValue, key.ToDeviceId(), nil, noCache)
	if err != nil {
		return nil, false, err
	}

	// when no cache, and we are the cloest closest node, return the data from local store
	if noCache && dht.IsTopNetwork && (len(nodes) == 0 || dht.Self.IsCloser(key.ToDeviceId(), nodes[0])) {
		dht.logger.Debugf("retrieve key %s, we are the closest node", key)
		storeData, exists := dht.store.Retrieve(key)
		if exists {
			return storeData.Value, true, nil
		} else {
			return nil, false, nil
		}
	}

	if storeData != nil {
		dht.logger.Debugf("retrieve key %s from closest node %s", key, nodes[0].Id)
		exist = true
		data = storeData.Value
		dht.storeLocal(key, data, false, storeData.Options)
	}

	return data, exist, nil
}

// Get retrieves data from the networking using key. Key is the base58 encoded
// identifier of the data.
func (dht *DHT) Retrieve(key deviceid.DataKey, noCache bool) (data []byte, found bool, err error) {
	dht.logger.Debugf("retrieve key %s", key)

	if !dht.isValidKey(key) {
		return nil, false, fmt.Errorf("invalid key %s", key)
	}

	if !noCache {
		storeData, exists := dht.store.Retrieve(key)
		if exists {
			dht.logger.Debugf("retrieve key %s from cache", key)
			return storeData.Value, true, nil
		}
	}
	_, _, _ = dht.singleflight.Do(key.ToDeviceId().String(), func() (interface{}, error) {
		data, found, err = dht.retrieve(key, noCache)
		return nil, err
	})

	return
}

func (dht *DHT) Update(key deviceid.DataKey, oldData, newData []byte, opts *idht.DHTStoreOptions) error {
	dht.logger.Debugf("update key %s", key)

	if !dht.isValidKey(key) {
		return fmt.Errorf("invalid key %s", key)
	}

	d, found, err := dht.Get(key, true)
	if err != nil {
		return err
	}

	if !found {
		// if not found and oldData is nil, then we can do a put
		if oldData == nil {
			return dht.Store(key, newData, opts)
		} else {
			return fmt.Errorf("key %s not exists", key)
		}
	}

	if !bytes.Equal(d, oldData) {
		return fmt.Errorf("key %s olData not equal", key)
	}

	// @todo: support compare when remote node try store it with old data hash, maybe introduce Update operation
	return dht.Store(key, newData, opts)
}

func (dht *DHT) Get(key deviceid.DataKey, noCache bool) (data []byte, found bool, err error) {
	return dht.Retrieve(key, noCache)
}

func (dht *DHT) Delete(key deviceid.DataKey) error {
	if !dht.isValidKey(key) {
		return fmt.Errorf("invalid key %s", key)
	}
	dht.logger.Debugf("delete key %s", key)
	dht.store.Delete(key)
	_, _, err := dht.iterate(iterateStore, key.ToDeviceId(), nil, &idht.DHTStoreOptions{})
	return err
}

func (dht *DHT) IterateBucketIdx(filter func(*device_info.DeviceInfo) bool, callback func(int, int, *device_info.DeviceInfo) error) {
	filterFunc := func(node *Node) bool {
		if filter == nil {
			return true
		}

		return filter(node.DeviceInfo)
	}

	applyBucketOneNodeFunc := func(idx int, bucketCnt int, node *Node) (next bool) {
		return callback(idx, bucketCnt, node.DeviceInfo) != nil
	}

	applyBucketAllNodesFunc := func(idx int, bucketCnt int, node *Node) (next bool) {
		_ = callback(idx, bucketCnt, node.DeviceInfo)
		return true
	}

	dht.ht.IterateNodes(filterFunc, applyBucketOneNodeFunc)

	if dht.IsTopNetwork {
		dht.gt.IterateNodes(filterFunc, applyBucketOneNodeFunc)
	} else {
		dht.gt.IterateNodes(filterFunc, applyBucketAllNodesFunc)
	}
}

func (dht *DHT) SetDeviceInfo(devInfo *device_info.DeviceInfo) {
	dht.logger.Debugf("SetDeviceInfo %+v", devInfo)
	if !devInfo.IsValid() {
		dht.logger.Errorf("invalid device info %+v", devInfo)
		return
	}
	dht.logger = dht.logger.WithField("id", devInfo.Id)
	self := NewNode(devInfo)
	dht.DeviceInfo = devInfo
	dht.Self = self
	dht.ht.Self = self
	dht.gt.Self = self
	dht.networking.SetNode(self)
	dht.networking.SetLogger(dht.logger)
}

func (dht *DHT) GetClosestTopNetworkDeviceInfo() *device_info.DeviceInfo {
	closestTopNetworkNode := dht.closestTopNetworkNode
	if closestTopNetworkNode != nil {
		return closestTopNetworkNode.DeviceInfo
	}

	return nil
}

func (dht *DHT) AddPeer(info *device_info.DeviceInfo) {
	dht.logger.Debugf("add peer %+v", info)
	dht.options.BootstrapNodes = append(dht.options.BootstrapNodes, NewNode(info))
	if dht.NumNodes(true) == 0 {
		dht.doBootstrap()
	}
}

func (dht *DHT) getNodeLocal(id deviceid.DeviceId) *Node {
	if id.IsTopNetwork() {
		return dht.ht.getNode(id)
	} else {
		return dht.gt.getNode(id)
	}
}

func (dht *DHT) GetDeviceInfo(id deviceid.DeviceId) *device_info.DeviceInfo {
	if id.IsEmpty() {
		return nil
	}

	node := dht.getNodeLocal(id)
	if node != nil {
		return node.DeviceInfo
	}

	_, nodes, _ := dht.iterate(iterateFindNode, id, nil, nil)
	if len(nodes) > 0 {
		firstNodeInfo := nodes[0].GetDeviceInfo()
		if firstNodeInfo.Id.Equal(id) {
			return firstNodeInfo
		}
	}
	return nil
}

func (dht *DHT) GetNeighborDeviceInfo(num int) []*device_info.DeviceInfo {
	if num <= 0 {
		return nil
	}

	var neighbors []*device_info.DeviceInfo = nil
	leftCount := num
	// get neighbor from ht first
	htNodes := dht.ht.getClosestContacts(num, dht.Self.Id, nil)
	for _, node := range htNodes.Nodes() {
		neighbors = append(neighbors, node.DeviceInfo)
		leftCount--
		if leftCount == 0 {
			break
		}
	}

	// get neighbor from gt
	if leftCount > 0 {
		gtNodes := dht.gt.getClosestContacts(leftCount, dht.Self.Id, nil)
		for _, node := range gtNodes.Nodes() {
			neighbors = append(neighbors, node.DeviceInfo)
			leftCount--
			if leftCount == 0 {
				break
			}
		}
	}

	return neighbors
}

func (dht *DHT) IterateClosestNode(
	targetId deviceid.DeviceId,
	filter func(*device_info.DeviceInfo) bool,
	callback func(*device_info.DeviceInfo) error,
) error {
	filterFunc := func(node *Node) bool {
		if filter == nil {
			return true
		}

		return filter(node.DeviceInfo)
	}
	_, closest, err := dht.iterate(iterateFindNode, targetId, nil, nil)
	if err != nil {
		return err
	}
	for _, node := range closest {
		if filter == nil || filterFunc(node) {
			next := callback(node.DeviceInfo) != nil
			if !next {
				return nil
			}
		}
	}
	return fmt.Errorf("not found closeest node")
}
