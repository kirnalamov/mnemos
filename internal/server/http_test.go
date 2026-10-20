package server

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kirnalamov/mnemos/internal/storage"
)

type fakeEngine struct {
	objects   map[string][]byte
	putErr    error
	getErr    error
	deleteErr error
}

func newFakeEngine() *fakeEngine {
	return &fakeEngine{
		objects: make(map[string][]byte),
	}
}

func (f *fakeEngine) Put(key string, data []byte) error {
	if f.putErr != nil {
		return f.putErr
	}
	f.objects[key] = bytes.Clone(data)
	return nil
}

func (f *fakeEngine) Get(key string) ([]byte, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	data, ok := f.objects[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return bytes.Clone(data), nil
}

func (f *fakeEngine) Delete(key string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.objects[key]; !ok {
		return storage.ErrNotFound
	}
	delete(f.objects, key)
	return nil
}

func TestHandlerPutAndGet(t *testing.T) {
	engine := newFakeEngine()
	handler := NewHandler(engine)

	body := []byte("hello, mnemos")
	putRequest := httptest.NewRequest(http.MethodPut, "/objects/greeting", bytes.NewReader(body))
	putResponse := httptest.NewRecorder()
	handler.ServeHTTP(putResponse, putRequest)

	if putResponse.Code != http.StatusCreated {
		t.Fatalf("PUT status = %d, want %d", putResponse.Code, http.StatusCreated)
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/objects/greeting", nil)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)

	if getResponse.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", getResponse.Code, http.StatusOK)
	}

	if got := getResponse.Body.Bytes(); !bytes.Equal(got, body) {
		t.Fatalf("GET body = %q, want %q", got, body)
	}

	if got := getResponse.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("Content-Type = %q, want application/octet-stream", got)
	}
}

func TestHandlerDelete(t *testing.T) {
	engine := newFakeEngine()
	handler := NewHandler(engine)

	engine.objects["temporary"] = []byte("data")

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/objects/temporary", nil)
	deleteResponse := httptest.NewRecorder()
	handler.ServeHTTP(deleteResponse, deleteRequest)

	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want %d", deleteResponse.Code, http.StatusNoContent)
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/objects/temporary", nil)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)

	if getResponse.Code != http.StatusNotFound {
		t.Fatalf("GET after DELETE status = %d, want %d", getResponse.Code, http.StatusNotFound)
	}
}

func TestHandlerNotFoundAndInvalidPath(t *testing.T) {
	handler := NewHandler(newFakeEngine())

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{
			name:       "missing object",
			method:     http.MethodGet,
			path:       "/objects/missing",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "empty key",
			method:     http.MethodGet,
			path:       "/objects/",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "nested key",
			method:     http.MethodGet,
			path:       "/objects/a/b",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "wrong route",
			method:     http.MethodGet,
			path:       "/other/key",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "unsupported method",
			method:     http.MethodPost,
			path:       "/objects/key",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.path, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
			if tt.name == "unsupported method" {
				if got := response.Header().Get("Allow"); got != "GET, PUT, DELETE" {
					t.Errorf("Allow header = %q, want %q", got, "GET, PUT, DELETE")
				}
			}
		})
	}
}

func TestHandlerPutEmptyObject(t *testing.T) {
	handler := NewHandler(newFakeEngine())

	request := httptest.NewRequest(http.MethodPut, "/objects/empty", bytes.NewReader(nil))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("PUT status = %d, want %d", response.Code, http.StatusCreated)
	}
}

func TestReadRequestBodyError(t *testing.T) {
	request := httptest.NewRequest(http.MethodPut, "/objects/key", nil)
	request.Body = failingReadCloser{}

	_, err := readRequestBody(request)
	if err == nil {
		t.Fatal("readRequestBody() error = nil, want non-nil")
	}
}

type failingReadCloser struct{}

func (failingReadCloser) Read([]byte) (int, error) {
	return 0, errors.New("simulated read error")
}

func (failingReadCloser) Close() error {
	return nil
}

var _ io.ReadCloser = failingReadCloser{}

func TestHandlerPutRejectsOversizedObject(t *testing.T) {
	handler := NewHandler(newFakeEngine())

	body := bytes.NewReader(make([]byte, (16<<20)+1))
	request := httptest.NewRequest(http.MethodPut, "/objects/large", body)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf(
			"status = %d, want %d",
			response.Code,
			http.StatusRequestEntityTooLarge,
		)
	}
}

func TestHandlerEngineErrors(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		setup  func(*fakeEngine)
		want   int
	}{
		{
			name:   "put error",
			method: http.MethodPut,
			path:   "/objects/key",
			setup: func(f *fakeEngine) {
				f.putErr = errors.New("simulated storage write failure")
			},
			want: http.StatusInternalServerError,
		},
		{
			name:   "get error",
			method: http.MethodGet,
			path:   "/objects/key",
			setup: func(f *fakeEngine) {
				f.getErr = errors.New("simulated storage read failure")
			},
			want: http.StatusInternalServerError,
		},
		{
			name:   "delete error",
			method: http.MethodDelete,
			path:   "/objects/key",
			setup: func(f *fakeEngine) {
				f.deleteErr = errors.New("simulated storage delete failure")
			},
			want: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeEngine()
			tt.setup(f)
			handler := NewHandler(f)

			request := httptest.NewRequest(tt.method, tt.path, bytes.NewReader([]byte("data")))
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != tt.want {
				t.Fatalf("status = %d, want %d", response.Code, tt.want)
			}
		})
	}
}
