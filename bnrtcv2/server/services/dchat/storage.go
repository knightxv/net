package dchat

import (
	"fmt"

	"github.com/guabee/bnrtc/bnrtcv2/store"
)

type StoreMessage = []byte

type Storage struct {
	Store   store.IStore
	Address string
	Count   uint32
	StoreID uint32
}

func newStorage(st store.IStore, address string) *Storage {
	return &Storage{
		Store:   st,
		Address: address,
	}
}

func (m *Storage) GetKey(id uint32) []byte {
	// @note max Uint32 is 4294967295, padding to 10 digits with `0`, for string sort
	return []byte(fmt.Sprintf("%s%s-%010d", DchatServiceStoreMessagePrefix, m.Address, id))
}

func (m *Storage) VirtualPush(idx uint32) {
	if m.Count == DchatServiceMessageLimit {
		key := m.GetKey(idx - DchatServiceMessageLimit)
		_ = m.Store.DeleteOne(key)
		m.Count--
	}
	m.Count++
	if m.StoreID < idx {
		m.StoreID = idx
	}
}

func (m *Storage) Push(msg []byte) {
	if msg == nil {
		return
	}

	key := m.GetKey(m.StoreID + 1)
	err := m.Store.PutOne(key, msg)
	if err == nil {
		m.VirtualPush(m.StoreID + 1)
	}
}

func (m *Storage) Delete(keys []store.StoreKey) {
	m.Store.Delete(keys)
}

func (m *Storage) Empty() bool {
	return m.Count == 0
}

func byteCopy(data []byte) []byte {
	newData := make([]byte, len(data))
	copy(newData, data)
	return newData
}

func (m *Storage) GetAll() ([]store.StoreKey, [][]byte) {
	messages := make([]StoreMessage, 0)
	keys := make([]store.StoreKey, 0)
	keyPrefix := fmt.Sprintf("%s%s-", DchatServiceStoreMessagePrefix, m.Address)
	_ = m.Store.Load(keyPrefix, func(key, val []byte) error {
		messages = append(messages, byteCopy(val))
		keys = append(keys, byteCopy(key))
		return nil
	})

	return keys, messages
}

func (m *Storage) Clear() {
	keyPrefix := fmt.Sprintf("%s%s-", DchatServiceStoreMessagePrefix, m.Address)
	m.Count = 0
	m.StoreID = 0
	_ = m.Store.Clear(keyPrefix)
}

type StorageManager struct {
	store    store.IStore
	storages map[string]*Storage
}

func NewStorageManager(store store.IStore) *StorageManager {
	return &StorageManager{
		store:    store,
		storages: make(map[string]*Storage),
	}
}

func (sm *StorageManager) HasStorage(address string) bool {
	_, ok := sm.storages[address]
	return ok
}

func (sm *StorageManager) CreateStorage(address string) *Storage {
	storage := newStorage(sm.store, address)
	sm.storages[address] = storage
	return storage
}

func (sm *StorageManager) GetStorage(address string) *Storage {
	storage, ok := sm.storages[address]
	if !ok {
		return nil
	}

	return storage
}

func (sm *StorageManager) GetOrCreateStorage(address string) *Storage {
	storage := sm.GetStorage(address)
	if storage == nil {
		storage = sm.CreateStorage(address)
	}

	return storage
}

func (sm *StorageManager) Push(address string, msg []byte) {
	storage := sm.GetOrCreateStorage(address)
	storage.Push(msg)
}

func (sm *StorageManager) Delete(address string, keys []store.StoreKey) {
	storage := sm.GetStorage(address)
	if storage != nil {
		storage.Delete(keys)
	}
}

func (sm *StorageManager) VirtualPush(address string, idx uint32) {
	storage := sm.GetOrCreateStorage(address)
	storage.VirtualPush(idx)
}

func (sm *StorageManager) GetMessages(address string) ([]store.StoreKey, [][]byte) {
	storage := sm.GetStorage(address)
	if storage == nil {
		return nil, nil
	}

	return storage.GetAll()
}

func (sm *StorageManager) Remove(address string) {
	storage := sm.GetStorage(address)
	if storage != nil {
		storage.Clear()
		delete(sm.storages, address)
	}
}
