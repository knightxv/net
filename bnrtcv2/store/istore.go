package store

// return error for abort
type IStoreLoadHandler func(key, val []byte) error

type StoreKey = []byte
type StoreValue = []byte

type StoreItem struct {
	Key StoreKey
	Val StoreValue
}

type IStore interface {
	Open(path string) error
	Load(prefix string, handler IStoreLoadHandler) error
	Put(items []*StoreItem) error
	PutOne(key StoreKey, val StoreValue) error
	Delete(keys []StoreKey) error
	DeleteOne(key StoreKey) error
	Clear(prefix string) error
	Close() error
}
