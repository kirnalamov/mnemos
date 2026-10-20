package server

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kirnalamov/mnemos/internal/storage"
)

type objectEngine interface {
	Put(key string, data []byte) error
	Get(key string) ([]byte, error)
	Delete(key string) error
}

type Handler struct {
	engine objectEngine
}

func NewHandler(engine objectEngine) *Handler {
	return &Handler{engine: engine}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/objects" || !strings.HasPrefix(r.URL.Path, "/objects/") {
		http.NotFound(w, r)
		return
	}

	key := strings.TrimPrefix(r.URL.Path, "/objects/")
	if key == "" || strings.Contains(key, "/") {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodPut:
		r.Body = http.MaxBytesReader(w, r.Body, 16<<20)

		data, err := readRequestBody(r)
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				http.Error(w, "object exceeds maximum size of 16 MiB", http.StatusRequestEntityTooLarge)
				return
			}

			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}

		if err := h.engine.Put(key, data); err != nil {
			http.Error(w, "failed to store object", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)

	case http.MethodGet:
		data, err := h.engine.Get(key)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "failed to read object", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)

	case http.MethodDelete:
		if err := h.engine.Delete(key); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "failed to delete object", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		w.Header().Set("Allow", "GET, PUT, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func readRequestBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}

func NewHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
}
