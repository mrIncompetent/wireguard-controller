package key

import (
	"sync"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func New() *Store {
	return &Store{
		m: &sync.RWMutex{},
	}
}

type Store struct {
	m   *sync.RWMutex
	key wgtypes.Key
}

func (s *Store) Set(key wgtypes.Key) {
	s.m.Lock()
	defer s.m.Unlock()

	s.key = key
}

func (s *Store) Get() wgtypes.Key {
	s.m.RLock()
	defer s.m.RUnlock()

	return s.key
}

func (s *Store) HasKey() bool {
	var emptyKey wgtypes.Key

	return s.key.String() != emptyKey.String()
}
