package full_url_rewrite_traefik_plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestSuccessfulInitialization(t *testing.T) {
	config := &Config{
		Regex:       "http(s)://example\\.(com|org)",
		Replacement: "https://example.com/path",
	}

	_, err := New(context.TODO(), nil, config, "test")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestInvalidRegexpInitialization(t *testing.T) {
	config := &Config{
		Regex:       "[",
		Replacement: "Something",
	}

	_, err := New(context.TODO(), nil, config, "test")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestServeHTTPSuccessfullyRewritesRequestUrl(t *testing.T) {
	config := &Config{
		Regex:       "https://",
		Replacement: "http://",
	}

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check the new request URL is what we expect.
		if r.URL.String() != "http://example.com/hello" {
			t.Errorf("handler returned unexpected URL: expected %v, got %v", "http://example.com/hello", r.URL.String())
		}

		_, _ = w.Write([]byte("OK"))
	})

	plugin, _ := New(context.TODO(), testHandler, config, "test")
	req, _ := http.NewRequest("GET", "https://example.com/hello", nil)

	plugin.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: expected %v, got %v", http.StatusOK, status)
	}
}

func TestServeHTTPMapsRewriteErrorToInternalServerError(t *testing.T) {
	config := &Config{
		Regex:       "https://",
		Replacement: ":/",
	}

	rr := httptest.NewRecorder()
	plugin, _ := New(context.TODO(), nil, config, "test")
	req, _ := http.NewRequest("GET", "https://example.com/hello", nil)

	plugin.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: expected %v, got %v", http.StatusInternalServerError, status)
	}
}

func TestURLRewrite(t *testing.T) {
	cases := []struct {
		name        string
		originalUrl string
		regex       string
		replacement string
		expectedUrl string
		expectedErr string
	}{
		{
			name:        "Simple string replacement",
			originalUrl: "https://example.com/hello",
			regex:       "hello",
			replacement: "goodbye",
			expectedUrl: "https://example.com/goodbye",
			expectedErr: "",
		},
		{
			name:        "Simple string no match",
			originalUrl: "https://example.com/hello",
			regex:       "goodbye",
			replacement: "something-else",
			expectedUrl: "https://example.com/hello",
			expectedErr: "",
		},
		{
			name:        "Updated URL is invalid",
			originalUrl: "https://example.com/hello",
			regex:       "https://",
			replacement: ":/",
			expectedUrl: "https://example.com/hello",
			expectedErr: "error parsing new URL \":/example.com/hello\": parse \":/example.com/hello\": missing protocol scheme",
		},
		{
			name:        "Regex replacement: remove query parameters",
			originalUrl: "https://example.com/hello?param=234&another=123",
			regex:       "(.+)\\?(.+)",
			replacement: "$1",
			expectedUrl: "https://example.com/hello",
			expectedErr: "",
		},
		{
			name:        "Regex replacement: replace path prefix with company name from subdomain 1",
			originalUrl: "https://cust-company1.example.com/prefix/hello",
			regex:       "https://(cust-(\\w+))\\.example\\.com/prefix/(.+)",
			replacement: "https://$1.example.com/$2/$3",
			expectedUrl: "https://cust-company1.example.com/company1/hello",
			expectedErr: "",
		},
		{
			name:        "Regex replacement: replace path prefix with company name from subdomain 2",
			originalUrl: "https://cust-company1.example.com/prefix/hello",
			regex:       "(https?)://cust-(\\w+)(.+)prefix/(.+)",
			replacement: "$1://cust-$2$3$2/$4",
			expectedUrl: "https://cust-company1.example.com/company1/hello",
			expectedErr: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tc.originalUrl, nil)
			rule := &rewriteRule{
				regexp:      regexp.MustCompile(tc.regex),
				replacement: tc.replacement,
			}

			err := rewriteRequestUrl(req, rule)
			if err != nil && err.Error() != tc.expectedErr {
				t.Fatalf("expected error to be: %v, got: %v", tc.expectedErr, err)
			}

			updatedUrl := req.URL.String()

			if updatedUrl != tc.expectedUrl {
				t.Fatalf("expected URL to be: %v, got: %v", tc.expectedUrl, updatedUrl)
			}
		})
	}
}
