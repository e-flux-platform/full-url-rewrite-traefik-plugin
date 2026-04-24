package full_url_rewrite_traefik_plugin

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
)

type Config struct {
	SourceStringFromHeader string `json:"sourceStringFromHeader,omitempty" toml:"sourceStringFromHeader,omitempty" yaml:"sourceStringFromHeader,omitempty"`
	Regex                  string `json:"regex,omitempty"                  toml:"regex,omitempty"                  yaml:"regex,omitempty"`
	Replacement            string `json:"replacement,omitempty"            toml:"replacement,omitempty"            yaml:"replacement,omitempty"`
}

func CreateConfig() *Config {
	return &Config{}
}

type rewriteRule struct {
	sourceStringFromHeader string
	regexp                 *regexp.Regexp
	replacement            string
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
		sourceStringFromHeader: config.SourceStringFromHeader,
		regexp:                 regexp,
		replacement:            config.Replacement,
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

// getReplacementSource returns the replacement source which is either taken from the header or the original request URL.
func getReplacementSource(headerName string, req *http.Request) []string {
	// If header name is provided, return the value of the header
	if headerName != "" {
		canonicalHeaderName := http.CanonicalHeaderKey(headerName)

		// Return the header value from the request if it exists
		if headerValue, ok := req.Header[canonicalHeaderName]; ok {
			return headerValue
		}
	}

	// Otherwise, return original request URL
	// Clone the URL to avoid mutating the original request
	originalUrlCopy := *req.URL

	// Grab the Host from the request as it's not included in the URL
	// since we're in the context of server request (we're acting as a proxy)
	// and in such case URL only contains Path and RawQuery (see RFC 7230, Section 5.3).
	originalUrlCopy.Host = req.Host

	return []string{originalUrlCopy.String()}
}

// replaceInSource replaces the first matching string in the source with the replacement string
// and returns the resulting string and a boolean indicating if a replacement was made.
func replaceInSource(source []string, rule *rewriteRule) (string, bool) {
	for _, item := range source {
		// Only replace the string if it matches the regex
		if rule.regexp.MatchString(item) {
			return rule.regexp.ReplaceAllString(item, rule.replacement), true
		}
	}

	return "", false
}

// rewriteRequestUrl rewrites request URL according to the given rule
// and returns new request instance if the URL has been updated.
func rewriteRequestUrl(originalRequest *http.Request, rule *rewriteRule) (*http.Request, error) {
	replacementSource := getReplacementSource(rule.sourceStringFromHeader, originalRequest)

	// Attempt to rewrite the strings from the replacement source to produce the new URL:
	if newURLStr, ok := replaceInSource(replacementSource, rule); ok {
		// Attempt to parse the new URL:
		newURL, err := url.Parse(newURLStr)
		if err != nil {
			return nil, fmt.Errorf("error initializing request with new URL %q: %w", newURLStr, err)
		}
		// Mutate the original request, rather than recreating, to persist metadata:
		originalRequest.URL = newURL
		originalRequest.RequestURI = originalRequest.URL.RequestURI()
	}

	return originalRequest, nil
}
