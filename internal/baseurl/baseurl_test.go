package baseurl

import (
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	t.Parallel()
	const (
		host   = "webservices5.autotask.net"
		secret = "s3cretpw"
		prefix = "autotask: test URL "
	)
	tests := []struct {
		name    string
		rawURL  string
		wantErr bool
	}{
		{"empty", "", true},
		{"no scheme", host + "/atservicesrest", true},
		{"other scheme", "ftp://" + host, true},
		{"no host", "https://", true},
		{"port without host", "http://:8080", true},
		{"trailing newline", "https://" + host + "\n", true},
		{"query", "https://" + host + "/atservicesrest?x=1", true},
		{"empty query", "https://" + host + "/atservicesrest?", true},
		{"fragment", "https://" + host + "/atservicesrest#x", true},
		{"user info", "https://apiuser:" + secret + "@" + host + "/atservicesrest/", true},
		{"user info, missing slash", "https:/apiuser:" + secret + "@" + host, true},
		{"user info, parse failure", "https://apiuser:" + secret + "@" + host + ":abc", true},
		{"password with slash", "https://apiuser:" + secret + "/x@" + host + "/", true},
		{"password with hash", "https://apiuser:" + secret + "#x@" + host + "/", true},
		{"digit password with slash", "https://apiuser:1234/" + secret + "@" + host + "/", true},
		{"https with path and trailing slash", "https://" + host + "/atservicesrest/", false},
		{"http with port, path and trailing slash", "http://127.0.0.1:1/atservicesrest/", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Check("test URL", tt.rawURL)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("Check(%q) = %v; want nil", tt.rawURL, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Check(%q) = nil; want an error", tt.rawURL)
			}
			if strings.Contains(err.Error(), secret) {
				t.Errorf("error %q repeats the password from the URL", err)
			}
			if !strings.HasPrefix(err.Error(), prefix) {
				t.Errorf("error %q does not start with %q", err, prefix)
			}
		})
	}
}
