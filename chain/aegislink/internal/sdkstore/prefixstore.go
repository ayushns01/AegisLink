package sdkstore

import (
	"encoding/json"
	"errors"
	"strings"

	storetypes "cosmossdk.io/store/types"
)

var ErrInvalidPrefix = errors.New("prefix is required")

const prefixStoreMetaPrefix = "_meta"
const prefixStoreInitializedKey = "initialized"
var prefixStoreInitializedValue = []byte("1")

type JSONPrefixStore struct {
	multiStore storetypes.CommitMultiStore
	key        *storetypes.KVStoreKey
}

func NewJSONPrefixStore(multiStore storetypes.CommitMultiStore, key *storetypes.KVStoreKey) (*JSONPrefixStore, error) {
	if multiStore == nil {
		return nil, ErrNilMultiStore
	}
	if key == nil {
		return nil, ErrNilStoreKey
	}

	return &JSONPrefixStore{
		multiStore: multiStore,
		key:        key,
	}, nil
}

func (s *JSONPrefixStore) HasAny(prefix string) bool {
	iter := storetypes.KVStorePrefixIterator(s.multiStore.GetKVStore(s.key), prefixBytes(prefix))
	defer iter.Close()
	return iter.Valid()
}

func (s *JSONPrefixStore) Load(prefix, id string, target any) (bool, error) {
	raw := s.multiStore.GetKVStore(s.key).Get(composePrefixedKey(prefix, id))
	if len(raw) == 0 {
		return false, nil
	}
	return true, json.Unmarshal(raw, target)
}

func (s *JSONPrefixStore) LoadAll(prefix string, newValue func() any, visit func(id string, value any) error) error {
	iter := storetypes.KVStorePrefixIterator(s.multiStore.GetKVStore(s.key), prefixBytes(prefix))
	defer iter.Close()

	prefixToken := string(prefixBytes(prefix))
	for ; iter.Valid(); iter.Next() {
		value := newValue()
		if err := json.Unmarshal(iter.Value(), value); err != nil {
			return err
		}
		id := strings.TrimPrefix(string(iter.Key()), prefixToken)
		if err := visit(id, value); err != nil {
			return err
		}
	}
	return nil
}

func (s *JSONPrefixStore) Save(prefix, id string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.multiStore.GetKVStore(s.key).Set(composePrefixedKey(prefix, id), encoded)
	return nil
}

func (s *JSONPrefixStore) Delete(prefix, id string) error {
	s.multiStore.GetKVStore(s.key).Delete(composePrefixedKey(prefix, id))
	return nil
}

func (s *JSONPrefixStore) ClearPrefix(prefix string) error {
	store := s.multiStore.GetKVStore(s.key)
	iter := storetypes.KVStorePrefixIterator(store, prefixBytes(prefix))
	defer iter.Close()

	var keys [][]byte
	for ; iter.Valid(); iter.Next() {
		keys = append(keys, append([]byte(nil), iter.Key()...))
	}
	for _, key := range keys {
		store.Delete(key)
	}
	return nil
}

func (s *JSONPrefixStore) Commit() error {
	s.ensureInitialized()
	s.multiStore.Commit()
	return nil
}

func (s *JSONPrefixStore) ensureInitialized() {
	store := s.multiStore.GetKVStore(s.key)
	initKey := composePrefixedKey(prefixStoreMetaPrefix, prefixStoreInitializedKey)
	if len(store.Get(initKey)) != 0 {
		return
	}
	store.Set(initKey, prefixStoreInitializedValue)
}

func prefixBytes(prefix string) []byte {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		return []byte{}
	}
	return []byte(trimmed + "/")
}

func composePrefixedKey(prefix, id string) []byte {
	return append(prefixBytes(prefix), []byte(strings.TrimSpace(id))...)
}
