# ML-DSA vectors from NIST's ACVP (FIPS 204)

The `*_prompt.json` and `*_expected.json` files are NIST's ACVP test vectors for ML-DSA (revision FIPS204), copied
from [usnistgov/ACVP-Server](https://github.com/usnistgov/ACVP-Server) at commit `975de31eb83d87039ec88934fdc47d8c312b892d`
(`gen-val/json-files/ML-DSA-{keyGen,sigGen,sigVer}-FIPS204`, public domain). Each prompt holds the inputs and each expected
file the outputs, with the same `tgId` and `tcId` values, as ACVP does; only the test groups and tests listed below are kept.

- `keyGen`: the first 10 tests of each parameter set (seed, public key, expanded private key).
- `sigGen`: the first 3 tests of each group that is not a HashML-DSA group (`preHash`): pure and internal signing, with
  the message representative `mu` or the formatted message `M'`, deterministic and randomized (`rnd`), from the
  expanded private key `sk`.
- `sigVer`: the first 5 tests of each group that is not HashML-DSA: pure verification with a context, internal `M'`
  and external `mu`, valid and invalid (`testPassed`).

The HashML-DSA groups (the `preHash` field) are not checked: Go's `crypto/mldsa` does not expose that mode, and neither
does `nist`. The full files are in the ACVP repository at that commit.

`bench/ref/mldsa_vectors` copies these files into case lines with their answers, and `tools/ci/mldsa_check.tin` feeds them to `tools/ci/fixtures/mldsa.tin`. The Wycheproof ML-DSA files
are in `toolchain/tests/wycheproof/mldsa/`, and `go_corpus.txt` (this directory) is the golden corpus of
`bench/ref/mldsa`, Go's answers from `crypto/mldsa` (Go 1.27).
