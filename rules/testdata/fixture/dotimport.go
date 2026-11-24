package fixture

import . "net/url"

type DotURL struct {
	Opts Options
	URL
}

var (
	_ = DotURL{URL: URL{Host: "x"}, Opts: Options{A: 1}} // want "directly: DotURL.* instead of URL: URL"
	_ = DotURL{Opts: Options{A: 1}, URL: URL{Host: "x"}}
)
