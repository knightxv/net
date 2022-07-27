package kaddht

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/config"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/dht/idht"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/store"
	"github.com/guabee/bnrtc/util"
)

type StoreData struct {
	Value       []byte                `json:"Value"`
	IsPublisher bool                  `json:"IsPublisher"`
	Options     *idht.DHTStoreOptions `json:"Options"`
}

type Store interface {
	// Store should store a key/value pair for the local node with the
	// given replication and expiration times.
	Store(key deviceid.DataKey, data []byte, replication time.Time, expiration time.Time, isPublisher bool, options *idht.DHTStoreOptions) error

	// Retrieve should return the local key/value if it exists.
	Retrieve(key deviceid.DataKey) (data *StoreData, found bool)

	// Delete should delete a key/value pair from the Store
	Delete(key deviceid.DataKey)

	// GetAllKeysForReplication should return the keys of all data to be
	// replicated across the network. Typically all data should be
	// replicated every tReplicate seconds.
	GetAllKeysForReplication(onlyPublisher bool) []deviceid.DataKey

	// ExpireKeys should expire all key/values due for expiration.
	ExpireKeys()
	KeyNum() int
	CheckOnBlacklist(key deviceid.DataKey) bool
}

// MemoryStore is a simple in-memory key/value store used for unit testing, and
// the CLI example
type MemoryStore struct {
	mutex        sync.Mutex
	dataMap      map[string]*StoreData
	replicateMap map[string]time.Time
	expireMap    map[string]time.Time
	blacklist    map[string]time.Time
	logger       *log.Entry
	db           store.IStore
}

func newMemoryStore(dhtMemoryStoreOption *config.DHTMemoryStoreOptions) *MemoryStore {
	mempryStore := &MemoryStore{
		dataMap:      make(map[string]*StoreData),
		replicateMap: make(map[string]time.Time),
		expireMap:    make(map[string]time.Time),
		blacklist:    make(map[string]time.Time),
		logger:       log.NewLoggerEntry("dht-store"),
	}
	// recovery form store
	if dhtMemoryStoreOption != nil && dhtMemoryStoreOption.UseDbStore {
		db := store.NewLevelStore()
		mempryStore.db = db
		err := mempryStore.db.Open(dhtMemoryStoreOption.StorePath)
		if err != nil {
			mempryStore.logger.Errorf("dht MemoryStore open db fail, err:%s", err)
		}

		{
			err := db.Load("data-", func(key, val []byte) error {
				storeKey := string(key)
				mapKey := strings.TrimPrefix(storeKey, "data-")
				if mapKey == "" {
					return nil
				}

				var storeData StoreData
				err := json.Unmarshal(val, &storeData)
				if err == nil {
					mempryStore.dataMap[mapKey] = &storeData
				}
				return nil
			})
			if err != nil {
				mempryStore.logger.Errorf("dht MemoryStore load data fail, err:%s", err)
			}
		}

		{
			err := db.Load("replicate-", func(key, val []byte) error {
				storeKey := string(key)
				mapKey := strings.TrimPrefix(storeKey, "replicate-")
				if mapKey == "" {
					return nil
				}

				var tm time.Time
				err := tm.GobDecode(val)
				if err == nil {
					mempryStore.replicateMap[mapKey] = tm
				}
				return nil
			})
			if err != nil {
				mempryStore.logger.Errorf("dht MemoryStore load replicate fail, err:%s", err)
			}
		}

		{
			err := db.Load("expire-", func(key, val []byte) error {
				storeKey := string(key)
				mapKey := strings.TrimPrefix(storeKey, "expire-")
				if mapKey == "" {
					return nil
				}

				var tm time.Time
				err := tm.GobDecode(val)
				if err == nil {
					mempryStore.expireMap[mapKey] = tm
				}
				return nil
			})
			if err != nil {
				mempryStore.logger.Errorf("dht MemoryStore load expire fail, err:%s", err)
			}
		}

		{
			err := db.Load("blacklist-", func(key, val []byte) error {
				storeKey := string(key)
				mapKey := strings.TrimPrefix(storeKey, "blacklist-")
				if mapKey == "" {
					return nil
				}

				var tm time.Time
				err := tm.GobDecode(val)
				if err == nil {
					mempryStore.blacklist[mapKey] = tm
				}
				return nil
			})
			if err != nil {
				mempryStore.logger.Errorf("dht MemoryStore load blacklist fail, err:%s", err)
			}
		}
	}

	return mempryStore
}

// GetAllKeysForReplication should return the keys of all data to be
// replicated across the network. Typically all data should be
// replicated every tReplicate seconds.
func (ms *MemoryStore) GetAllKeysForReplication(onlyPublisher bool) []deviceid.DataKey {
	ms.mutex.Lock()
	var keys []deviceid.DataKey
	for k, d := range ms.dataMap {
		if d.Options != nil && d.Options.NoReplicate {
			continue
		}

		if onlyPublisher && !d.IsPublisher {
			continue
		}
		if time.Now().After(ms.replicateMap[k]) {
			keys = append(keys, deviceid.DataKey(k))
		}
	}
	ms.mutex.Unlock()
	return keys
}

// ExpireKeys should expire all key/values due for expiration.
func (ms *MemoryStore) ExpireKeys() {
	ms.mutex.Lock()
	for k, v := range ms.expireMap {
		if time.Now().After(v) {
			delete(ms.replicateMap, k)
			delete(ms.expireMap, k)
			delete(ms.dataMap, k)
			err := ms.DBDelete(k)
			if err != nil {
				ms.logger.Debugf("dht MemoryStore delete ExpireKeys fail, key:%s, err:%s", k, err)
			}
		}
	}
	ms.mutex.Unlock()
}

// Store will store a key/value pair for the local node with the given
// replication and expiration times.
func (ms *MemoryStore) Store(key deviceid.DataKey, data []byte, replication time.Time, expiration time.Time, isPublisher bool, options *idht.DHTStoreOptions) error {
	if data == nil {
		ms.Delete(key)
		return nil
	}

	ms.mutex.Lock()
	strKey := string(key)
	ms.replicateMap[strKey] = replication
	ms.expireMap[strKey] = expiration
	storeData := &StoreData{
		Value:       data,
		IsPublisher: isPublisher,
		Options:     options,
	}
	ms.dataMap[strKey] = storeData
	// save to level db
	err := ms.DBStore(strKey, replication, expiration, storeData)
	if err != nil {
		ms.logger.Debugf("dht MemoryStore store fail, key:%s, err:%s", strKey, err)
	}
	ms.mutex.Unlock()
	return nil
}

// Retrieve will return the local key/value if it exists
func (ms *MemoryStore) Retrieve(key deviceid.DataKey) (data *StoreData, found bool) {
	ms.mutex.Lock()
	data, found = ms.dataMap[string(key)]
	ms.mutex.Unlock()
	return
}

// Delete deletes a key/value pair from the MemoryStore
func (ms *MemoryStore) Delete(key deviceid.DataKey) {
	ms.mutex.Lock()
	strKey := string(key)
	delete(ms.replicateMap, strKey)
	delete(ms.expireMap, strKey)
	delete(ms.dataMap, strKey)
	// delete to level db
	err := ms.DBDelete(strKey)
	if err != nil {
		ms.logger.Debugf("dht MemoryStore delete fail, key:%s, err:%s", strKey, err)
	}
	ms.mutex.Unlock()
}

func (ms *MemoryStore) KeyNum() int {
	return len(ms.dataMap)
}

func (ms *MemoryStore) AddBlacklist(key deviceid.DataKey, expireTime time.Duration) error {
	blackTimeout := time.Now().Add(expireTime)
	strKey := string(key)
	ms.blacklist[strKey] = blackTimeout
	if ms.db != nil {
		dbKey := []byte("blacklist-" + strKey)
		dbVal, err := blackTimeout.GobEncode()
		if err == nil {
			ms.db.PutOne(dbKey, dbVal)
		}
	}
	return nil
}

func (ms *MemoryStore) deleteBlacklist(key deviceid.DataKey) error {
	strKey := string(key)
	delete(ms.blacklist, strKey)
	if ms.db != nil {
		dbKey := []byte("blacklist-" + strKey)
		return ms.db.DeleteOne(dbKey)
	}
	return nil
}

func (ms *MemoryStore) CheckOnBlacklist(key deviceid.DataKey) bool {
	strKey := string(key)
	expireTime, found := ms.blacklist[strKey]
	if !found {
		return false
	}
	if time.Now().After(expireTime) {
		defer func() {
			if ms.db != nil {
				dbKey := []byte("blacklist-" + strKey)
				ms.db.DeleteOne(dbKey)
			}
		}()
		return false
	}
	return true
}

func (ms *MemoryStore) DBDelete(key string) error {
	if ms.db == nil {
		return fmt.Errorf("db is nil, can not be option")
	}
	return ms.db.Delete([]store.StoreKey{
		util.Str2bytes("replicate-" + key),
		util.Str2bytes("expire-" + key),
		util.Str2bytes("data-" + key),
	})
}

func (ms *MemoryStore) DBStore(
	key string,
	replication time.Time,
	expiration time.Time,
	storeData *StoreData,
) error {
	if ms.db == nil {
		return fmt.Errorf("db is nil, can not be option")
	}
	replData, _ := replication.GobEncode()
	expData, _ := expiration.GobEncode()
	storeDataByte, _ := json.Marshal(storeData)
	return ms.db.Put([]*store.StoreItem{
		{Key: util.Str2bytes("replicate-" + key), Val: replData},
		{Key: util.Str2bytes("expire-" + key), Val: expData},
		{Key: util.Str2bytes("data-" + key), Val: storeDataByte},
	})
}

func (ms *MemoryStore) DBClose() error {
	if ms.db == nil {
		return fmt.Errorf("db is nil, can not be option")
	}
	return ms.db.Close()
}
