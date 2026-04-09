package sdkstore

import (
	"encoding/json"
	"errors"

	storetypes "cosmossdk.io/store/types"
)

var (
	ErrNilMultiStore = errors.New("commit multi-store is required")
	ErrNilStoreKey   = errors.New("store key is required")
)

var stateKey = []byte("state")

type JSONStateStore struct {
	multiStore storetypes.CommitMultiStore
	key        *storetypes.KVStoreKey
}

func NewJSONStateStore(multiStore storetypes.CommitMultiStore, key *storetypes.KVStoreKey) (*JSONStateStore, error) {
	if multiStore == nil {
		return nil, ErrNilMultiStore
	}
	if key == nil {
		return nil, ErrNilStoreKey
	}

	return &JSONStateStore{
		multiStore: multiStore,
		key:        key,
	}, nil
}

func (s *JSONStateStore) Load(target any) error {
	value := s.multiStore.GetKVStore(s.key).Get(stateKey)
	if len(value) == 0 {
		return nil
	}
	return json.Unmarshal(value, target)
}

func (s *JSONStateStore) HasState() bool {
	value := s.multiStore.GetKVStore(s.key).Get(stateKey)
	return len(value) > 0
}

func (s *JSONStateStore) Save(value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}

	s.multiStore.GetKVStore(s.key).Set(stateKey, encoded)
	s.multiStore.Commit()
	return nil
}
