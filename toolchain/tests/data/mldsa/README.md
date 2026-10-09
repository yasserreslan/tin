# ML-DSA golden corpus (`go_corpus.txt`)

`go_corpus.txt` is the golden corpus of `bench/ref/mldsa` (#917): one case per line in the fixture's input form
(`tools/ci/fixtures/mldsa.tin`), a tab, and the class Go gives it: `ok`, `fault`, `sha256:` and the digest of the bytes
Go returned, or `hex:` and the value (for `decompose`, the FNV-prime hash of Decompose over every value below q).

Go's answers come from `crypto/mldsa` of Go 1.27, and the `decompose` answers from the definition of Decompose
(FIPS 204 Algorithm 36) in the same program. The corpus holds keys and deterministic signatures (the seeds, messages
and contexts are derived from SHA-256 in `bench/ref/mldsa/main.go`), the verifications of those signatures and of
tampered and malformed inputs, the pre-hashed mu mode, and three randomized signatures of Go's own, pinned as bytes.

Regenerate with `go run ./bench/ref/mldsa > toolchain/tests/data/mldsa/go_corpus.txt` (Go 1.27 or later). Regeneration
changes the three randomized lines; `sh tools/ci/tin.sh mldsa_check` checks the file in any Go, and with Go 1.27 it also
checks that the live Go twin still gives every class in it (`go run ./bench/ref/mldsa classes`).
