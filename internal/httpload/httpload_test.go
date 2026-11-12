package httpload

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchReturnsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("openapi: 3.0.3\n"))
	}))
	defer srv.Close()

	data, err := Fetch(srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(data) != "openapi: 3.0.3\n" {
		t.Errorf("body = %q", data)
	}
}

func TestFetchRejectsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := Fetch(srv.URL)
	if err == nil {
		t.Fatal("expected an error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error %q does not include the status", err)
	}
}

func TestFetchFollowsRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.RedirectHandler(target.URL, http.StatusFound))
	defer redirect.Close()

	data, err := Fetch(redirect.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(data) != "ok" {
		t.Errorf("body = %q, want ok", data)
	}
}

func TestFetchBadURL(t *testing.T) {
	if _, err := Fetch("://nope"); err == nil {
		t.Fatal("expected an error for a malformed URL")
	}
}
