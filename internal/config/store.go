package config

import "sync/atomic"

// ConfigStore holds the active Config and allows atomic updates.
// Jobs and workers always call Load() to get the latest version.
type ConfigStore struct {
	ptr atomic.Pointer[Config]
}

func NewConfigStore(cfg *Config) *ConfigStore {
	s := &ConfigStore{}
	s.ptr.Store(cfg)
	return s
}

func (s *ConfigStore) Load() *Config {
	return s.ptr.Load()
}

func (s *ConfigStore) Store(cfg *Config) {
	s.ptr.Store(cfg)
}
