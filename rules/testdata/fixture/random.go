package fixture

import (
	"math/rand/v2"
	"time"
)

// fakeRand has an Int64N method but is not *rand.Rand.
type fakeRand struct{}

func (*fakeRand) Int64N(n int64) int64 { return n - 1 }

func randomN(r *rand.Rand, f *fakeRand, n int64, d time.Duration, u uint32, i int, w uint64, k uint) []any {
	return []any{
		int64(r.Int64N(int64(n))),         // want "use r.N.n. instead of converting through a fixed-width bounded method"
		time.Duration(r.Int64N(int64(d))), // want "use r.N.d. instead"
		uint32(r.Uint32N(uint32(u))),      // want "use r.N.u. instead"
		int64(r.Int32N(int32(n))),         // want "use r.N.n. instead"
		int(r.IntN(int(i))),               // want "use r.N.i. instead"
		uint64(r.Uint64N(uint64(w))),      // want "use r.N.w. instead"
		uint(r.UintN(uint(k))),            // want "use r.N.k. instead"
		rand.Int64N(int64(n)),
		int64(rand.Int64N(int64(n))),
		int(r.Int64N(int64(n))),
		int64(f.Int64N(int64(n))),
	}
}
