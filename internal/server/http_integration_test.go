package server

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/kirnalamov/mnemos/internal/engine"
)

func TestHTTPIntegrationObjectLifecycle(t *testing.T) {
	dataPath := filepath.Join(t.TempDir(), "data.mnemos")

	e, err := engine.Open(dataPath)
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}

	ts := httptest.NewServer(NewHandler(e))
	defer ts.Close()
	defer func() {
		if err := e.Close(); err != nil {
			t.Errorf("close engine: %v", err)
		}
	}()

	client := ts.Client()
	url := ts.URL + "/objects/greeting"
	firstBody := []byte("hello, mnemos")

	put := doRequest(t, client, http.MethodPut, url, firstBody)
	put.Body.Close()
	if put.StatusCode != http.StatusCreated {
		t.Fatalf("PUT status = %d, want %d", put.StatusCode, http.StatusCreated)
	}

	get := doRequest(t, client, http.MethodGet, url, nil)
	got, err := io.ReadAll(get.Body)
	get.Body.Close()
	if err != nil {
		t.Fatalf("read GET body: %v", err)
	}
	if get.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", get.StatusCode, http.StatusOK)
	}
	if !bytes.Equal(got, firstBody) {
		t.Fatalf("GET body = %q, want %q", got, firstBody)
	}

	secondBody := []byte("updated content")
	put = doRequest(t, client, http.MethodPut, url, secondBody)
	put.Body.Close()
	if put.StatusCode != http.StatusCreated {
		t.Fatalf("overwrite PUT status = %d, want %d", put.StatusCode, http.StatusCreated)
	}

	get = doRequest(t, client, http.MethodGet, url, nil)
	got, err = io.ReadAll(get.Body)
	get.Body.Close()
	if err != nil {
		t.Fatalf("read GET body after overwrite: %v", err)
	}
	if get.StatusCode != http.StatusOK || !bytes.Equal(got, secondBody) {
		t.Fatalf("GET after overwrite: status = %d, body = %q", get.StatusCode, got)
	}

	del := doRequest(t, client, http.MethodDelete, url, nil)
	del.Body.Close()
	if del.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want %d", del.StatusCode, http.StatusNoContent)
	}

	get = doRequest(t, client, http.MethodGet, url, nil)
	get.Body.Close()
	if get.StatusCode != http.StatusNotFound {
		t.Fatalf("GET after DELETE status = %d, want %d", get.StatusCode, http.StatusNotFound)
	}
}

func TestHTTPIntegrationPersistsObjectAfterReopen(t *testing.T) {
	dataPath := filepath.Join(t.TempDir(), "data.mnemos")
	body := []byte("persistent object")

	e, err := engine.Open(dataPath)
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}

	ts := httptest.NewServer(NewHandler(e))
	put := doRequest(t, ts.Client(), http.MethodPut, ts.URL+"/objects/persist", body)
	put.Body.Close()
	ts.Close()

	if put.StatusCode != http.StatusCreated {
		_ = e.Close()
		t.Fatalf("PUT status = %d, want %d", put.StatusCode, http.StatusCreated)
	}
	if err := e.Close(); err != nil {
		t.Fatalf("close engine: %v", err)
	}

	e, err = engine.Open(dataPath)
	if err != nil {
		t.Fatalf("reopen engine: %v", err)
	}
	defer func() {
		if err := e.Close(); err != nil {
			t.Errorf("close reopened engine: %v", err)
		}
	}()

	ts = httptest.NewServer(NewHandler(e))
	defer ts.Close()

	get := doRequest(t, ts.Client(), http.MethodGet, ts.URL+"/objects/persist", nil)
	defer get.Body.Close()

	got, err := io.ReadAll(get.Body)
	if err != nil {
		t.Fatalf("read GET body after reopen: %v", err)
	}
	if get.StatusCode != http.StatusOK {
		t.Fatalf("GET after reopen status = %d, want %d", get.StatusCode, http.StatusOK)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("GET after reopen body = %q, want %q", got, body)
	}
}

func TestHTTPIntegrationRejectsShortBody(t *testing.T) {
	dataPath := filepath.Join(t.TempDir(), "data.mnemos")

	e, err := engine.Open(dataPath)
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}
	defer func() {
		if err := e.Close(); err != nil {
			t.Errorf("close engine: %v", err)
		}
	}()

	handler := NewHandler(e)
	request := httptest.NewRequest(
		http.MethodPut,
		"/objects/short",
		bytes.NewReader([]byte("abc")),
	)
	request.ContentLength = 5

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"PUT status = %d, want %d; body = %q",
			response.Code,
			http.StatusBadRequest,
			response.Body.String(),
		)
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/objects/short", nil)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)

	if getResponse.Code != http.StatusNotFound {
		t.Fatalf(
			"GET status = %d, want %d",
			getResponse.Code,
			http.StatusNotFound,
		)
	}
}

func doRequest(t *testing.T, client *http.Client, method, url string, body []byte) *http.Response {
	t.Helper()

	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create %s request: %v", method, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s request failed: %v", method, err)
	}
	return resp
}
