package redact

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestURL(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, raw, want string }{
		{"query dropped", "https://h.example/a/b?user=secret&x=1", "https://h.example/a/b"},
		{"user info dropped", "https://name:pw@h.example/a", ""},
		{"fragment dropped", "https://h.example/a#frag", "https://h.example/a"},
		{"at sign in the path", "https://user:1234/x@host/", ""},
		{"unparsable", "https://h.example/%zz", ""},
		{"empty query marker dropped", "https://h.example/a?", "https://h.example/a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := URL(tt.raw); got != tt.want {
				t.Errorf("URL(%q) = %q; want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestURLErrorRewritesTheWrappedURLError(t *testing.T) {
	t.Parallel()
	inner := &url.Error{Op: "Get", URL: "https://h.example/z?user=secret", Err: errors.New("EOF")}
	if got := URLError(errors.Join(errors.New("outer"), inner)); got == nil {
		t.Fatal("URLError must return its argument")
	}
	if strings.Contains(inner.Error(), "secret") {
		t.Errorf("url.Error text = %q; want no query", inner.Error())
	}
	if plain := errors.New("plain"); !errors.Is(URLError(plain), plain) {
		t.Error("an error without a *url.Error must be returned unchanged")
	}
}
