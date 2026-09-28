package redirect

import (
	"errors"
	"net/http"
	"net/url"
	"testing"
)

func TestSameOrigin(t *testing.T) {
	t.Parallel()
	const base = "https://webservices2.autotask.net/ATServicesRest/"
	tests := []struct {
		name    string
		rawURL  string
		baseURL string
		want    bool
	}{
		{"same host, other path", "https://webservices2.autotask.net/v1.0/Tickets", base, true},
		{"host differs only in case", "https://WebServices2.Autotask.net/v1.0/Tickets", base, true},
		{"Unicode fold of a host letter", "https://webservices2.autotas\u212A.net/v1.0/Tickets", base, false},
		{"other host", "https://evil.example/v1.0/Tickets", base, false},
		{"other host, same length", "https://webservices3.autotask.net/v1.0/Tickets", base, false},
		{"suffix lookalike", "https://webservices2.autotask.net.evil.com/v1.0/Tickets", base, false},
		{"scheme downgrade", "http://webservices2.autotask.net/v1.0/Tickets", base, false},
		{"explicit port", "https://webservices2.autotask.net:8443/v1.0/Tickets", base, false},
		{"unparseable URL", "https://webservices2.autotask.net/%zz", base, false},
		{"unparseable base", "https://webservices2.autotask.net/v1.0", "https://webservices2.autotask.net/%zz", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := SameOrigin(tt.rawURL, tt.baseURL); got != tt.want {
				t.Errorf("SameOrigin(%q, %q) = %v; want %v", tt.rawURL, tt.baseURL, got, tt.want)
			}
		})
	}
}

// redirectRequest builds the request net/http would pass to CheckRedirect,
// with every credential header already copied onto it.
func redirectRequest(t *testing.T, rawURL string) *http.Request {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	req := &http.Request{URL: u, Header: http.Header{}}
	for _, h := range CredentialHeaders {
		req.Header.Set(h, "value")
	}
	return req
}

func TestGuard(t *testing.T) {
	t.Parallel()
	const base = "https://api.example"

	t.Run("cross-origin removes credentials", func(t *testing.T) {
		t.Parallel()
		req := redirectRequest(t, "https://other.example/x")
		if err := Guard(&http.Client{}, base).CheckRedirect(req, nil); err != nil {
			t.Fatalf("CheckRedirect error = %v; want nil", err)
		}
		for _, h := range CredentialHeaders {
			if got := req.Header.Get(h); got != "" {
				t.Errorf("%s = %q; want empty", h, got)
			}
		}
	})

	t.Run("same-origin keeps credentials", func(t *testing.T) {
		t.Parallel()
		req := redirectRequest(t, base+"/moved")
		if err := Guard(&http.Client{}, base).CheckRedirect(req, nil); err != nil {
			t.Fatalf("CheckRedirect error = %v; want nil", err)
		}
		for _, h := range CredentialHeaders {
			if got := req.Header.Get(h); got != "value" {
				t.Errorf("%s = %q; want %q", h, got, "value")
			}
		}
	})

	t.Run("default limit", func(t *testing.T) {
		t.Parallel()
		hc := &http.Client{}
		guarded := Guard(hc, base)
		req := redirectRequest(t, base+"/loop")
		if err := guarded.CheckRedirect(req, make([]*http.Request, maxRedirects-1)); err != nil {
			t.Errorf("hop %d error = %v; want nil", maxRedirects, err)
		}
		if err := guarded.CheckRedirect(req, make([]*http.Request, maxRedirects)); err == nil {
			t.Errorf("hop %d error = nil; want the redirect limit error", maxRedirects+1)
		}
		if hc.CheckRedirect != nil {
			t.Error("Guard set CheckRedirect on the input client; want it left nil")
		}
	})

	t.Run("delegates to caller", func(t *testing.T) {
		t.Parallel()
		errStop := errors.New("stop")
		var sawSecret string
		hc := &http.Client{CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			sawSecret = req.Header.Get("Secret")
			return errStop
		}}
		req := redirectRequest(t, "https://other.example/x")
		// The caller's func replaces the default limit, so a long chain still
		// reaches it.
		err := Guard(hc, base).CheckRedirect(req, make([]*http.Request, maxRedirects+5))
		if !errors.Is(err, errStop) {
			t.Errorf("CheckRedirect error = %v; want the caller's error", err)
		}
		if sawSecret != "" {
			t.Errorf("caller saw Secret = %q; want it removed first", sawSecret)
		}
	})
}
