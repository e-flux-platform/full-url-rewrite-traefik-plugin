package full_url_rewrite_traefik_plugin

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSuccessfulInitialization(t *testing.T) {
	config := &Config{
		Regex:       "//example\\.(com|org)",
		Replacement: "//example.com/path",
	}

	_, err := New(t.Context(), nil, config, "test")
	require.NoError(t, err)
}

func TestInvalidRegexpInitialization(t *testing.T) {
	config := &Config{
		Regex:       "[",
		Replacement: "Something",
	}

	_, err := New(t.Context(), nil, config, "test")
	require.Error(t, err)
}

func TestServeHTTPSuccessfullyRewritesRequestUrl(t *testing.T) {
	config := &Config{
		Regex:       "/hello",
		Replacement: "/world",
	}

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "//example.com/world", r.URL.String())
		_, _ = w.Write([]byte("OK"))
	})

	plugin, err := New(t.Context(), testHandler, config, "test")
	require.NoError(t, err)
	req, err := http.NewRequest("GET", "//example.com/hello", nil)
	require.NoError(t, err)

	plugin.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestServeHTTPMapsRewriteErrorToInternalServerError(t *testing.T) {
	config := &Config{
		Regex:       "//",
		Replacement: ":/",
	}

	rr := httptest.NewRecorder()
	plugin, err := New(t.Context(), nil, config, "test")
	require.NoError(t, err)
	req, err := http.NewRequest("GET", "//example.com/hello", nil)
	require.NoError(t, err)

	plugin.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestURLRewrite(t *testing.T) {
	cases := []struct {
		name                   string
		originalUrl            string
		headers                map[string]string
		sourceStringFromHeader string
		regex                  string
		replacement            string
		expectedUrl            string
		expectedErr            string
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
		{
			name:        "Regex replacement: take the source string from header",
			originalUrl: "//example.com/hello?param=234&another=123",
			headers: map[string]string{
				"Host":            "example.com",
				"X-Original-Host": "another-company.com",
			},
			sourceStringFromHeader: "X-Original-Host",
			regex:                  "^(.+)\\.com$",
			replacement:            "//example.com/$1",
			expectedUrl:            "//example.com/another-company",
			expectedErr:            "",
		},
		{
			name:        "Regex replacement: should not rewrite URL if no sources matched the regex",
			originalUrl: "//example.com/hello?param=234&another=123",
			headers: map[string]string{
				"Host":            "example.com",
				"X-Original-Host": "another-company.com",
			},
			sourceStringFromHeader: "X-Original-Host",
			regex:                  "^(.+)\\.nl$",
			replacement:            "//example.com/$1",
			expectedUrl:            "//example.com/hello?param=234&another=123",
			expectedErr:            "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", tc.originalUrl, nil)
			require.NoError(t, err)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			rule := &rewriteRule{
				sourceStringFromHeader: tc.sourceStringFromHeader,
				regexp:                 regexp.MustCompile(tc.regex),
				replacement:            tc.replacement,
			}

			newReq, err := rewriteRequestUrl(req, rule)
			if tc.expectedErr != "" {
				require.Error(t, err)
				assert.Equal(t, tc.expectedErr, err.Error())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedUrl, newReq.URL.String())
		})
	}
}

func TestGetBody(t *testing.T) {
	rule := &rewriteRule{
		regexp:      regexp.MustCompile("hello"),
		replacement: "goodbye",
	}

	const payload = "important"

	req, err := http.NewRequest("POST", "//example.com/hello", strings.NewReader(payload))
	require.NoError(t, err)
	require.NotNil(t, req.GetBody, "incoming client-style request exposes GetBody for reopening")

	out, err := rewriteRequestUrl(req, rule)
	require.NoError(t, err)
	assert.NotNil(t, out.GetBody)

	// First read:
	reader, err := out.GetBody()
	require.NoError(t, err)
	first, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, payload, string(first))

	// Second read:
	reader, err = out.GetBody()
	require.NoError(t, err)
	second, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, payload, string(second))
}

func TestContentLength(t *testing.T) {
	rule := &rewriteRule{
		regexp:      regexp.MustCompile("hello"),
		replacement: "goodbye",
	}

	const payload = "important"

	// Only io.Reader (wrapped by NopCloser), not *bytes.Buffer / *strings.Reader, so net/http
	// cannot infer length and uses -1—while the reverse proxy may already know Content-Length.
	opaque := struct{ io.Reader }{Reader: strings.NewReader(payload)}
	req, err := http.NewRequest("POST", "//example.com/hello", io.NopCloser(opaque))
	require.NoError(t, err)
	req.ContentLength = int64(len(payload))

	out, err := rewriteRequestUrl(req, rule)
	require.NoError(t, err)

	assert.Equal(t, req.ContentLength, out.ContentLength)
	assert.Equal(t, int64(len(payload)), out.ContentLength)
	assert.Nil(t, out.GetBody)
}
