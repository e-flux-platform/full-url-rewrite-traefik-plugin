package full_url_rewrite_traefik_plugin

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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
	if err := rewriteRequestUrl(req, fullUrlRewrite.rewriteRule); err != nil {
		http.Error(rw, fmt.Sprintf("error rewriting URL: %v", err), http.StatusInternalServerError)
		return
	}

	fullUrlRewrite.next.ServeHTTP(rw, req)
}

// rewriteRequestUrl mutates the the given request with new URL according to the given rule.
// If the resulting URL is invalid, the request URL is left unchanged.
func rewriteRequestUrl(request *http.Request, rule *rewriteRule) error {
	originalUrl := request.URL.String()

	newUrl := rule.regexp.ReplaceAllString(originalUrl, rule.replacement)

	if newUrl != originalUrl {
		newUrlParsed, err := url.Parse(newUrl)
		if err != nil {
			return fmt.Errorf("error parsing new URL %q: %w", newUrl, err)
		}

		request.URL = newUrlParsed
	}

	return nil
}
