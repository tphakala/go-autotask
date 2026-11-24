//go:build ruleguard

package gorules

import (
	"github.com/quasilyte/go-ruleguard/dsl"
	"github.com/quasilyte/go-ruleguard/dsl/types"
)

// MinMaxBuiltin detects manual min/max implementations using if statements
// or ternary-like patterns and suggests using the built-in min/max functions.
//
// Old patterns:
//
//	// If-based min
//	if a < b {
//	    result = a
//	} else {
//	    result = b
//	}
//
//	// math.Min/Max for integers
//	result := int(math.Min(float64(a), float64(b)))
//
// New pattern (Go 1.21+):
//
//	result := min(a, b)
//	result := max(a, b)
//
// Benefits:
//   - Cleaner, more readable code
//   - Works with any ordered type
//   - No type conversion needed
//
// See: https://pkg.go.dev/builtin#min
// See: https://pkg.go.dev/builtin#max
func MinMaxBuiltin(m dsl.Matcher) {
	// math.Min with float64 conversion for integers
	m.Match(
		`int(math.Min(float64($a), float64($b)))`,
	).
		Report("use min($a, $b) instead of int(math.Min(float64(...))) (Go 1.21+)").
		Suggest("min($a, $b)")

	m.Match(
		`int64(math.Min(float64($a), float64($b)))`,
	).
		Report("use min($a, $b) instead of int64(math.Min(float64(...))) (Go 1.21+)").
		Suggest("min($a, $b)")

	m.Match(
		`int32(math.Min(float64($a), float64($b)))`,
	).
		Report("use min($a, $b) instead of int32(math.Min(float64(...))) (Go 1.21+)").
		Suggest("min($a, $b)")

	// math.Max with float64 conversion for integers
	m.Match(
		`int(math.Max(float64($a), float64($b)))`,
	).
		Report("use max($a, $b) instead of int(math.Max(float64(...))) (Go 1.21+)").
		Suggest("max($a, $b)")

	m.Match(
		`int64(math.Max(float64($a), float64($b)))`,
	).
		Report("use max($a, $b) instead of int64(math.Max(float64(...))) (Go 1.21+)").
		Suggest("max($a, $b)")

	m.Match(
		`int32(math.Max(float64($a), float64($b)))`,
	).
		Report("use max($a, $b) instead of int32(math.Max(float64(...))) (Go 1.21+)").
		Suggest("max($a, $b)")
}

// ClearBuiltin detects loop-based map/slice clearing patterns and suggests
// using the built-in clear() function.
//
// Old patterns:
//
//	// Map clearing
//	for k := range m {
//	    delete(m, k)
//	}
//
//	// Slice zeroing
//	for i := range s {
//	    s[i] = 0  // or nil, "", etc.
//	}
//
// New pattern (Go 1.21+):
//
//	clear(m)  // Deletes all map entries
//	clear(s)  // Sets all slice elements to zero value
//
// Benefits:
//   - Cleaner, more readable code
//   - More efficient (optimized implementation)
//   - Works with maps and slices
//
// See: https://pkg.go.dev/builtin#clear
func ClearBuiltin(m dsl.Matcher) {
	// Map clearing pattern: for k := range m { delete(m, k) }
	m.Match(
		`for $k := range $m { delete($m, $k) }`,
	).
		Report("use clear($m) instead of loop-based map clearing (Go 1.21+)").
		Suggest("clear($m)")

	// Map clearing with underscore value: for k, _ := range m { delete(m, k) }
	m.Match(
		`for $k, _ := range $m { delete($m, $k) }`,
	).
		Report("use clear($m) instead of loop-based map clearing (Go 1.21+)").
		Suggest("clear($m)")
}

// RangeOverInteger detects traditional for loops that iterate from 0 to n
// and suggests using the Go 1.22+ range-over-integer syntax.
//
// Old pattern:
//
//	for i := 0; i < n; i++ {
//	    process(i)
//	}
//
// New pattern (Go 1.22+):
//
//	for i := range n {
//	    process(i)
//	}
//
// Benefits:
//   - More concise and readable
//   - Intent is clearer (iterate n times)
//   - Less error-prone (no off-by-one mistakes)
//
// Note: Only matches loops starting from 0 with < comparison and i++.
// Loops with different starting values, comparisons, or increments
// are intentionally not flagged. The bound $n must also be a plain
// identifier or dotted selector (x, obj.field): a call or len(...) bound
// is not flagged, because range evaluates its operand once whereas the
// classic loop re-evaluates the bound each iteration, so the rewrite is
// not always equivalent. The body must also not reassign $n or mutate the
// index $i or take its address, since range owns the index and evaluates
// its operand once. This also excludes b.N (see the .N filter below).
//
// See: https://go.dev/doc/go1.22#language
func RangeOverInteger(m dsl.Matcher) {
	// Pattern: for i := 0; i < n; i++
	// Exclude benchmark loops (b.N) which should use b.Loop() instead
	m.Match(
		`for $i := 0; $i < $n; $i++ { $*body }`,
	).
		Where(
			m["n"].Text.Matches(`^[A-Za-z_]\w*(\.[A-Za-z_]\w*)*$`) &&
				!m["n"].Text.Matches(`.*\.N$`) &&
				!m["body"].Contains(`$n = $_`) &&
				!m["body"].Contains(`$n += $_`) &&
				!m["body"].Contains(`$n -= $_`) &&
				!m["body"].Contains(`$n++`) &&
				!m["body"].Contains(`$n--`) &&
				!m["body"].Contains(`$i = $_`) &&
				!m["body"].Contains(`$i++`) &&
				!m["body"].Contains(`$i--`) &&
				!m["body"].Contains(`&$i`),
		).
		// Report only, no Suggest: a Suggest template containing the $*body group
		// does not round-trip through golangci-lint's gocritic --fix, which writes
		// the literal "$*body" into the source and corrupts the loop. Verified with
		// golangci-lint 2.13.2. Report the modernization; leave the edit to a human.
		Report("use for $i := range $n instead of for $i := 0; $i < $n; $i++ (Go 1.22+)")
}

// AppendWithoutValues detects append calls with no values which have no effect.
//
// Broken pattern:
//
//	slice = append(slice)  // No effect
//
// See: https://pkg.go.dev/builtin#append
// Note: Go 1.22 vet tool also warns about this pattern.
func AppendWithoutValues(m dsl.Matcher) {
	m.Match(
		`append($s)`,
	).
		Report("append with single argument has no effect; did you forget the values to append?")
}

// NewWithExpression detects the slice-literal hack for getting a pointer to a value
// and suggests using Go 1.26's enhanced new() built-in.
//
// Old pattern (slice hack):
//
//	field := &[]string{"hello"}[0]
//	field := &[]int{42}[0]
//	field := &[]time.Duration{5 * time.Second}[0]
//
// New pattern (Go 1.26+):
//
//	field := new("hello")
//	field := new(42)
//	field := new(5 * time.Second)
//
// Benefits:
//   - Eliminates the obscure slice-literal-index hack
//   - Clearer intent: "pointer to this value"
//   - No intermediate slice allocation
//   - Works with any expression, including function calls
//
// See: https://go.dev/doc/go1.26#language
func NewWithExpression(m dsl.Matcher) {
	// Pattern: &[]T{v}[0] - the well-known slice hack for pointer-to-value
	m.Match(
		`&[]$typ{$val}[0]`,
	).
		Report("use new($val) instead of &[]$typ{$val}[0] (Go 1.26+)").
		Suggest("new($val)")
}

// EmbeddedFieldLiteral detects a struct literal that initializes a promoted
// field through a nested literal of the embedded type, and suggests keying the
// promoted field directly, which Go 1.27 allows.
//
// Old pattern:
//
//	type Base struct{ Name string }
//	type Config struct {
//	    Base
//	    Port int
//	}
//	cfg := Config{Base: Base{Name: "x"}, Port: 80}
//
// New pattern (Go 1.27+):
//
//	cfg := Config{Name: "x", Port: 80}
//
// The spec now allows a struct literal key to be any valid field selector for
// a (possibly promoted) field of the struct, as long as no embedded field on the
// path is a pointer. This mirrors the embedlit modernizer in go fix.
//
// The rule fires when the key and the nested literal's type share a name,
// which is how an embedded value field looks in a literal, the nested literal's
// type is a struct, and the outer struct embeds a struct by value. A regular
// field named after its type, such as Header in http.Response, does not match,
// except when its type is a struct and the outer struct also embeds a struct by
// value: the filters see types, not which field the key names. A promoted name
// that is shadowed or ambiguous in the outer struct cannot be keyed directly,
// so this is a report rather than a rewrite.
//
// Each of the two patterns appears to bind only the first element of the
// literal that fits its own shape and to test the conditions on that element
// alone, so an embedded field keyed after another element can go unreported.
// The patterns differ by how the embedded type is spelled: the first expects a
// bare name (a same-package or dot-imported type) and its $embT also seems to
// fit pkg.Name, so it can stop at an earlier element whose value is any keyed
// literal; the second expects pkg.Name and fits only elements spelled that way.
// The cases measured with golangci-lint 2.15.0 are in
// rules/testdata/fixture.
//
// See: https://go.dev/doc/go1.27#language
func EmbeddedFieldLiteral(m dsl.Matcher) {
	m.Match(
		`$T{$*_, $emb: $embT{$k: $v, $*_}, $*_}`,
	).
		Where(m["emb"].Text == m["embT"].Text && m["embT"].Filter(isStructType) && m["T"].Filter(embedsStruct)).
		Report("initialize the promoted field directly: $T{$k: $v, ...} instead of $emb: $embT{$k: $v, ...} (Go 1.27+); not applicable if $k is shadowed or ambiguous in $T, or if $emb is a regular field named after its type")

	// Embedded type written as pkg.Name: field name is the bare type name.
	m.Match(
		`$T{$*_, $emb: $pkg.$embT{$k: $v, $*_}, $*_}`,
	).
		Where(m["emb"].Text == m["embT"].Text && m["embT"].Filter(isStructType) && m["T"].Filter(embedsStruct)).
		Report("initialize the promoted field directly: $T{$k: $v, ...} instead of $emb: $pkg.$embT{$k: $v, ...} (Go 1.27+); not applicable if $k is shadowed or ambiguous in $T, or if $emb is a regular field named after its type")
}

// isStructType reports whether a type is a struct type. The nested literal
// that initializes an embedded field through a promoted key is a struct.
func isStructType(ctx *dsl.VarFilterContext) bool {
	return types.AsStruct(ctx.Type.Underlying()) != nil
}

// embedsStruct reports whether the struct type of a composite literal embeds a
// struct by value. An embedded pointer cannot be keyed through, and an embedded
// interface or other non-struct type has no fields to promote. The loop has a
// condition only: quasigo, which runs ruleguard filters, has no three-clause
// for loop (go-ruleguard v0.4.5, ruleguard/quasigo/compile.go:226-228). A
// filter it cannot compile fails the load of the whole file
// (ruleguard/ir_loader.go:93). gocritic would skip every rule in the file
// (go-critic v0.15.0, checkers/ruleguard_checker.go:234-238) with no message a
// user sees, since golangci-lint v2.15.0 discards linter log output
// (pkg/commands/run.go:327-329), so .golangci.yaml sets failOn: all to make it
// a lint error; see the comment there about the lint cache.
func embedsStruct(ctx *dsl.VarFilterContext) bool {
	st := types.AsStruct(ctx.Type.Underlying())
	if st == nil {
		return false
	}
	n := st.NumFields()
	i := 0
	for i < n {
		f := st.Field(i)
		if f.Embedded() && types.AsStruct(f.Type().Underlying()) != nil {
			return true
		}
		i++
	}
	return false
}
