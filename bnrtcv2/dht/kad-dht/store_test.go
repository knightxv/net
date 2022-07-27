package kaddht

import (
	"bytes"
	"testing"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/config"
	"github.com/guabee/bnrtc/bnrtcv2/dht/idht"
)

func TestMemoryStore(t *testing.T) {

	storeOption := &config.DHTMemoryStoreOptions{
		StorePath:  "D:/bnrtcTestDb",
		UseDbStore: true,
	}

	store := newMemoryStore(storeOption)

	key := []byte{1, 2, 3}
	data := []byte{4, 5, 6}

	replication := time.Now()
	expiration := time.Now()
	options := &idht.DHTStoreOptions{
		TExpire:    time.Duration(time.Duration.Seconds(20)),
		TReplicate: time.Duration(time.Duration.Seconds(50)),
	}

	err := store.Store(key, data, replication, expiration, false, options)
	// store save func
	{
		if err != nil {
			t.Errorf("store key fail, err: %s", err)
		}
		_, found := store.Retrieve(key)
		if !found {
			t.Errorf("store key but not found")
		}
		store.DBClose()
	}
	// store recovery func
	{
		store := newMemoryStore(storeOption)
		storeData, found := store.Retrieve(key)
		if !found {
			t.Errorf("recovery form leveldb, but store key not found")
		}
		equal := bytes.Equal(storeData.Value, data)
		if !equal {
			t.Errorf("recovery form leveldb, but store value not equal save data")
		}
		store.DBClose()
	}
	// store db recovery func
	{
		store := newMemoryStore(storeOption)
		_, found := store.Retrieve(key)
		if !found {
			t.Errorf("recovery form leveldb, but store key not found")
		}
		store.Delete(key)
		{
			_, found := store.Retrieve(key)
			if found {
				t.Errorf("store key is deleted, but found still")
			}
		}
		store.DBClose()
		{
			store2 := newMemoryStore(storeOption)
			_, found := store2.Retrieve(key)
			if found {
				t.Errorf("delete key form leveldb, but still save in db")
			}
			store2.DBClose()
		}

	}
}

func TestBlackList(t *testing.T) {

	key := []byte{1, 2, 3}
	store := newMemoryStore(nil)

	{
		inBlackList := store.CheckOnBlacklist(key)
		if inBlackList {
			t.Errorf("not add key:%s in blacklist, but check on black list", string(key))
		}
	}

	store.AddBlacklist(key, time.Second)

	{
		inBlackList := store.CheckOnBlacklist(key)
		if !inBlackList {
			t.Errorf("add key:%s in blacklist, but not found in black list", string(key))
		}
	}
	time.Sleep(1 * time.Second)
	{
		inBlackList := store.CheckOnBlacklist(key)
		if inBlackList {
			t.Errorf("key:%s is expire, but still found in black list", string(key))
		}
	}

	storeOption := &config.DHTMemoryStoreOptions{
		StorePath:  "D:/bnrtcTestDb",
		UseDbStore: true,
	}

	{
		store := newMemoryStore(storeOption)
		store.AddBlacklist(key, 100*time.Second)
		inBlackList := store.CheckOnBlacklist(key)
		if !inBlackList {
			t.Errorf("add key:%s in blacklist, but not found in black list", string(key))
		}
		store.DBClose()
	}
	{
		store := newMemoryStore(storeOption)
		inBlackList := store.CheckOnBlacklist(key)
		if !inBlackList {
			t.Errorf("recovery key:%s in blacklist, but not found in black list", string(key))
		}
		store.DBClose()
	}

}
