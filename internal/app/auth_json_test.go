package app

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestOAuthResponseJSON(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantError bool
	}{
		{"valid", `{"access_token":"token","expires_in":60}`, false},
		{"duplicate key", `{"access_token":"token","access_token":"other","expires_in":60}`, true},
		{"invalid UTF-8", "{\"access_token\":\"\xff\",\"expires_in\":60}", true},
		{"case sensitive fields", `{"ACCESS_TOKEN":"token","expires_in":60}`, true},
		{"trailing value", `{"access_token":"token","expires_in":60} {}`, true},
		{"oversized response", `{"access_token":"` + strings.Repeat("x", 1<<20) + `","expires_in":60}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &tokenManager{config: Config{AuthTokenURL: "https://token.example"}, client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})}}
			err := m.refresh(t.Context())
			if (err != nil) != tc.wantError {
				t.Fatalf("refresh error = %v, want error %v", err, tc.wantError)
			}
			if !tc.wantError && m.token != "token" {
				t.Fatalf("token = %q", m.token)
			}
		})
	}
}
