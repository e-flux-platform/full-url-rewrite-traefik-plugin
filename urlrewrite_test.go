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
		Regex:       "//example\\.(com|org)",
		Replacement: "//example.com/path",
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
		Regex:       "/hello",
		Replacement: "/world",
	}

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check the new request URL is what we expect.
		if r.URL.String() != "//example.com/world" {
			t.Errorf("handler returned unexpected URL: expected %v, got %v", "//example.com/world", r.URL.String())
		}

		_, _ = w.Write([]byte("OK"))
	})

	plugin, _ := New(context.TODO(), testHandler, config, "test")
	req, _ := http.NewRequest("GET", "//example.com/hello", nil)

	plugin.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: expected %v, got %v", http.StatusOK, status)
	}
}

func TestServeHTTPMapsRewriteErrorToInternalServerError(t *testing.T) {
	config := &Config{
		Regex:       "//",
		Replacement: ":/",
	}

	rr := httptest.NewRecorder()
	plugin, _ := New(context.TODO(), nil, config, "test")
	req, _ := http.NewRequest("GET", "//example.com/hello", nil)

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
			originalUrl: "//example.com/hello",
			regex:       "hello",
			replacement: "goodbye",
			expectedUrl: "//example.com/goodbye",
			expectedErr: "",
		},
		{
			name:        "Simple string no match",
			originalUrl: "//example.com/hello",
			regex:       "goodbye",
			replacement: "something-else",
			expectedUrl: "//example.com/hello",
			expectedErr: "",
		},
		{
			name:        "Updated URL is invalid",
			originalUrl: "//example.com/hello",
			regex:       "//",
			replacement: ":/",
			expectedUrl: "//example.com/hello",
			expectedErr: "error initializing request with new URL \":/example.com/hello\": parse \":/example.com/hello\": missing protocol scheme",
		},
		{
			name:        "Regex replacement: remove query parameters",
			originalUrl: "//example.com/hello?param=234&another=123",
			regex:       "(.+)\\?(.+)",
			replacement: "$1",
			expectedUrl: "//example.com/hello",
			expectedErr: "",
		},
		{
			name:        "Regex replacement: replace path prefix with company name from subdomain 1",
			originalUrl: "//cust-company1.example.com/prefix/hello",
			regex:       "//(cust-(\\w+))\\.example\\.com/prefix/(.+)",
			replacement: "//$1.example.com/$2/$3",
			expectedUrl: "//cust-company1.example.com/company1/hello",
			expectedErr: "",
		},
		{
			name:        "Regex replacement: replace path prefix with company name from subdomain 2",
			originalUrl: "//cust-company1.example.com/prefix/hello",
			regex:       "//cust-(\\w+)(.+)prefix/(.+)",
			replacement: "//cust-$1$2$1/$3",
			expectedUrl: "//cust-company1.example.com/company1/hello",
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

			newReq, err := rewriteRequestUrl(req, rule)
			if err != nil {
				if err.Error() != tc.expectedErr {
					t.Fatalf("expected error to be: %v, got: %v", tc.expectedErr, err)
				}
				return
			}

			updatedUrl := newReq.URL.String()
			if updatedUrl != tc.expectedUrl {
				t.Fatalf("expected URL to be: %v, got: %v", tc.expectedUrl, updatedUrl)
				return
			}
		})
	}
}
