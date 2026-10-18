package engine

import "github.com/kirnalamov/mnemos/internal/storage"

type Engine struct {
	store *storage.Store
}

func Open(path string) (*Engine, error) {
	store, err := storage.Open(path)
	if err != nil {
		return nil, err
	}

	return &Engine{
		store: store,
	}, nil
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
