package store

import (
	"errors"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type LevelStore struct {
	db *leveldb.DB
}

func NewLevelStore() *LevelStore {
	return &LevelStore{}
}

func (s *LevelStore) Open(path string) error {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return err
	}

	s.db = db
	return nil
}

func (s *LevelStore) Load(prefix string, handler IStoreLoadHandler) error {
	if s.db == nil {
		return errors.New("db is nil")
	}

	iter := s.db.NewIterator(util.BytesPrefix([]byte(prefix)), nil)
	for iter.Next() {
		key := make([]byte, len(iter.Key()))
		copy(key, iter.Key())
		val := make([]byte, len(iter.Value()))
		copy(val, iter.Value())
		err := handler(iter.Key(), val)
		if err != nil {
			break
		}
	}
	iter.Release()
	return nil
}

func (s *LevelStore) Put(items []*StoreItem) error {
	if s.db == nil {
		return errors.New("db is nil")
	}

	if len(items) == 0 {
		return nil
	}

	batch := new(leveldb.Batch)
	for _, item := range items {
		batch.Put(item.Key, item.Val)
	}

	return s.db.Write(batch, nil)
}

func (s *LevelStore) PutOne(key StoreKey, val StoreValue) error {
	if s.db == nil {
		return errors.New("db is nil")
	}

	if len(key) == 0 {
		return errors.New("key is empty")
	}

	return s.db.Put(key, val, nil)
}

func (s *LevelStore) Delete(keys []StoreKey) error {
	if s.db == nil {
		return errors.New("db is nil")
	}

	if len(keys) == 0 {
		return nil
	}

	batch := new(leveldb.Batch)
	for _, key := range keys {
		batch.Delete(key)
	}

	return s.db.Write(batch, nil)
}

func (s *LevelStore) DeleteOne(key StoreKey) error {
	if s.db == nil {
		return errors.New("db is nil")
	}

	if len(key) == 0 {
		return errors.New("key is empty")
	}

	return s.db.Delete(key, nil)
}

func (s *LevelStore) Clear(prefix string) error {
	if s.db == nil {
		return errors.New("db is nil")
	}

	iter := s.db.NewIterator(util.BytesPrefix([]byte(prefix)), nil)
	batch := new(leveldb.Batch)
	for iter.Next() {
		batch.Delete(iter.Key())
	}
	iter.Release()
	if batch.Len() > 0 {
		return s.db.Write(batch, nil)
	}

	return nil
}

func (s *LevelStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}
