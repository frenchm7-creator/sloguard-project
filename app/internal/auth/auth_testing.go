package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBearer(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	h := Bearer("s3cret", ok)
	cases := map[string]int{"": 401, "Bearer nope": 401, "s3cret": 401, "Bearer s3cret": 200}
	for header, want := range cases {
		req := httptest.NewRequest("GET", "/metrics", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("header %q: got %d want %d", header, rec.Code, want)
		}
	}
	rec := httptest.NewRecorder()
	Bearer("", ok).ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	if rec.Code != 200 {
		t.Error("empty token must disable auth")
	}
}
