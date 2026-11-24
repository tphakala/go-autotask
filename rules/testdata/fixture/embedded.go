// Package fixture holds literals that the ruleguard rules in rules/*.go must
// or must not report. A line that must be reported ends in a want comment
// with a regular expression the report must match; every other line must not
// be reported. rules/check-fixtures.sh lints this package and compares.
package fixture

import (
	"io"
	"net/http"
)

type Base struct{ Name string }
type Other struct{ ID int }
type Options struct{ A int }
type Names map[string]int
type Hdr = http.Header

type Embeds struct{ Base }

// Late embeds Base after another field.
type Late struct {
	ID int
	Base
}

type EmbedsImported struct{ http.Client }

type Mixed struct {
	Other
	Base Base
}

type OptsClient struct {
	Opts Options
	http.Client
}

type MapClient struct {
	Labels map[string]string
	http.Client
}

type AliasClient struct {
	Header Hdr
	http.Client
}

type Regular struct{ Base Base }

type MixedIface struct {
	io.Reader
	Base Base
}

type MixedHeader struct {
	Other
	Header http.Header
}

type EmbedsPtr struct {
	*Other
	Base Base
}

type MixedNames struct {
	Other
	Names Names
}

type RegImported struct{ Client http.Client }

type MixedIfaceImported struct {
	io.Reader
	Client http.Client
}

type EmbedsPtrImported struct {
	*Other
	Client http.Client
}

type OptsFirst struct {
	Opts Options
	Base
}

type HdrBase struct {
	Header http.Header
	Base
}

// KeyedClient embeds Base by value and has a regular http.Client field whose
// name differs from its type.
type KeyedClient struct {
	Base
	Conn http.Client
}

type HdrClient struct {
	Header http.Header
	http.Client
}

var (
	_ = Embeds{Base: Base{Name: "x"}}                                                 // want "directly: Embeds.* instead of Base: Base"
	_ = Late{ID: 1, Base: Base{Name: "x"}}                                            // want "directly: Late.* instead of Base: Base"
	_ = EmbedsImported{Client: http.Client{Jar: nil}}                                 // want "directly: EmbedsImported.* instead of Client: http.Client"
	_ = OptsClient{Opts: Options{A: 1}, Client: http.Client{Jar: nil}}                // want "directly: OptsClient.* instead of Client: http.Client"
	_ = MapClient{Labels: map[string]string{"a": "b"}, Client: http.Client{Jar: nil}} // want "directly: MapClient.* instead of Client: http.Client"
	_ = AliasClient{Header: Hdr{"A": nil}, Client: http.Client{Jar: nil}}             // want "directly: AliasClient.* instead of Client: http.Client"

	// Known false positive: Base is a regular field named after its struct
	// type, and Mixed embeds Other by value. The filters see types, not which
	// field the key names (see the EmbeddedFieldLiteral comment).
	_ = Mixed{Base: Base{Name: "x"}} // want "directly: Mixed.* instead of Base: Base"

	_ = Regular{Base: Base{Name: "x"}}
	_ = &http.Response{Header: http.Header{"A": nil}}
	_ = MixedIface{Base: Base{Name: "x"}}
	_ = MixedHeader{Header: http.Header{"A": nil}}
	_ = EmbedsPtr{Base: Base{Name: "x"}}
	_ = MixedNames{Names: Names{"a": 1}}
	_ = RegImported{Client: http.Client{Jar: nil}}
	_ = MixedIfaceImported{Client: http.Client{Jar: nil}}
	_ = EmbedsPtrImported{Client: http.Client{Jar: nil}}
	_ = OptsFirst{Opts: Options{A: 1}, Base: Base{Name: "x"}}
	_ = HdrBase{Header: http.Header{"A": nil}, Base: Base{Name: "x"}}
	_ = HdrClient{Header: http.Header{"A": nil}, Client: http.Client{Jar: nil}}
	_ = KeyedClient{Conn: http.Client{Jar: nil}}
)
