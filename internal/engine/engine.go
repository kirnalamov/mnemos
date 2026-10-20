package engine

import "github.com/kirnalamov/mnemos/internal/storage"

type objectStore interface {
	Put(key string, data []byte) error
	Get(key string) ([]byte, error)
	Delete(key string) error
	Close() error
	Compact() error
}

type Engine struct {
	store objectStore
}

func Open(path string) (*Engine, error) {
	store, err := storage.Open(path)
	if err != nil {
		return nil, err
	}

	return &Engine{store: store}, nil
}

func (e *Engine) Put(key string, data []byte) error {
	return e.store.Put(key, data)
}

func (e *Engine) Get(key string) ([]byte, error) {
	return e.store.Get(key)
}

func (e *Engine) Delete(key string) error {
	return e.store.Delete(key)
}

func (e *Engine) Close() error {
	return e.store.Close()
}

func (e *Engine) Compact() error {
	return e.store.Compact()
}
