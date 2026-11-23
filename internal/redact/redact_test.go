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

func TestURLErrorReturnsARedactedCopy(t *testing.T) {
	t.Parallel()
	orig := &url.Error{Op: "Get", URL: "https://h.example/z?user=secret", Err: errors.New("EOF")}
	got := URLError(orig)
	if strings.Contains(got.Error(), "secret") {
		t.Errorf("error text = %q; want no query", got.Error())
	}
	if !strings.Contains(orig.Error(), "secret") {
		t.Error("the argument must not be modified")
	}
	if plain := errors.New("plain"); !errors.Is(URLError(plain), plain) {
		t.Error("an error without a *url.Error must be returned unchanged")
	}
}

func TestURLErrorRedactsNestedURLErrors(t *testing.T) {
	t.Parallel()
	inner := &url.Error{Op: "Get", URL: "https://other.example/x?user=bob&tok=SECRET", Err: errors.New("boom")}
	outer := &url.Error{Op: "Get", URL: "https://h.example/a?user=alice", Err: inner}
	got := URLError(outer).Error()
	for _, leak := range []string{"user=", "SECRET", "alice", "bob"} {
		if strings.Contains(got, leak) {
			t.Errorf("error text %q still contains %q", got, leak)
		}
	}
	if !strings.Contains(inner.Error(), "SECRET") {
		t.Error("the nested error must not be modified")
	}
}

func TestURLErrorHidesTheCauseOfAParseErrorWithUserInfo(t *testing.T) {
	t.Parallel()
	// With an unescaped "/" in the password, url.Parse reads "user:pw" as the
	// host and quotes ":pw" in the cause of its error.
	err := &url.Error{Op: "parse", URL: "https://user:pw/x@host/a", Err: errors.New(`invalid port ":pw" after host`)}
	if got := URLError(err).Error(); strings.Contains(got, "pw") || strings.Contains(got, "user") {
		t.Errorf("error text %q still quotes the user info", got)
	}
}

func TestURLErrorKeepsTheCauseOfAParseErrorWithoutUserInfo(t *testing.T) {
	t.Parallel()
	err := &url.Error{Op: "parse", URL: "https://h.example/%zz", Err: errors.New("invalid URL escape")}
	if got := URLError(err).Error(); !strings.Contains(got, "invalid URL escape") {
		t.Errorf("error text %q lost the cause", got)
	}
}
