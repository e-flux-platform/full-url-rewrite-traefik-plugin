package full_url_rewrite_traefik_plugin

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
)

type Config struct {
	Regex       string `json:"regex,omitempty"       toml:"regex,omitempty"       yaml:"regex,omitempty"`
	Replacement string `json:"replacement,omitempty" toml:"replacement,omitempty" yaml:"replacement,omitempty"`
}

func CreateConfig() *Config {
	return &Config{}
}

type rewriteRule struct {
	regexp      *regexp.Regexp
	replacement string
}

type FullUrlRewrite struct {
	next        http.Handler
	name        string
	rewriteRule *rewriteRule
}

// New creates a new FullUrlRewrite plugin.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	regexp, err := regexp.Compile(config.Regex)
	if err != nil {
		return nil, fmt.Errorf("%s: error compiling regex %q: %w", name, config.Regex, err)
	}

	rewriteRule := &rewriteRule{
		regexp:      regexp,
		replacement: config.Replacement,
	}

	return &FullUrlRewrite{
		next: next, rewriteRule: rewriteRule, name: name,
	}, nil
}

func (fullUrlRewrite *FullUrlRewrite) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	newReq, err := rewriteRequestUrl(req, fullUrlRewrite.rewriteRule)
	if err != nil {
		http.Error(rw, fmt.Sprintf("error rewriting URL: %v", err), http.StatusInternalServerError)
		return
	}

	fullUrlRewrite.next.ServeHTTP(rw, newReq)
}

// rewriteRequestUrl rewrites request URL according to the given rule
// and returns new request instance if the URL has been updated.
func rewriteRequestUrl(originalRequest *http.Request, rule *rewriteRule) (*http.Request, error) {
	// Clone the URL to avoid mutating the original request
	originalUrlCopy := *originalRequest.URL

	// Grab the Host from the request as it's not included in the URL
	// since we're in the context of server request (we're acting as a proxy)
	// and in such case URL only contains Path and RawQuery (see RFC 7230, Section 5.3).
	originalUrlCopy.Host = originalRequest.Host
	originalUrlStr := originalUrlCopy.String()
	newUrlStr := rule.regexp.ReplaceAllString(originalUrlStr, rule.replacement)

	if newUrlStr != originalUrlStr {
		// Create a new request with the new URL
		newRequest, err := http.NewRequestWithContext(
			originalRequest.Context(),
			originalRequest.Method,
			newUrlStr,
			originalRequest.Body,
		)
		if err != nil {
			return nil, fmt.Errorf("error initializing request with new URL %q: %w", newUrlStr, err)
		}

		newRequest.RequestURI = newRequest.URL.RequestURI()
		newRequest.Header = originalRequest.Header.Clone()
		newRequest.RemoteAddr = originalRequest.RemoteAddr

		return newRequest, nil
	}

	return originalRequest, nil
}
