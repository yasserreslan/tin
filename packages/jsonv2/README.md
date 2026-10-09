# jsonv2

encoding/json/v2's surface, built on `argo` (typed encoders and decoders) and `jsontext` (token-level text):
`Marshal` and `Unmarshal` with options (#931). The JSON token API is `toolchain/std/jsontext`.

## API

- `Marshal[T](v T, opts Options) !str`: the JSON of `v`, with no newline at the end (as Go's `Marshal`).
  The text is `argo.Put`'s, rewritten token by token with the options.
- `Unmarshal[T](s str, v mut T, opts Options) !`: parses `s` into `v` (a struct, slice or map). A fault leaves `v` unchanged.
- `Options`: `Multiline`, `Indent`, `IndentPrefix`, `SpaceAfterColon`, `SpaceAfterComma`, `EscapeForHTML`, `EscapeForJS`
  (the formatting and escapes of `Marshal`, as jsontext's); `AllowDuplicateNames` (`Unmarshal` accepts a repeated member name, the
  later value wins); `RejectUnknownMembers` (`Unmarshal` fails on a member that matches no field).

## Design

- `Marshal` encodes with `argo.Put`, which is compiled per type, then re-reads the text with `jsontext` and writes it with the
  options. `argo`'s HTML escapes are therefore removed unless `EscapeForHTML` asks for them, as v2 does by default.
- `Unmarshal` first reads every value with `jsontext` (syntax errors and duplicate names are reported in jsontext's words, as v2 does),
  then decodes with `argo.Get`, or `argo.GetStrict` with `RejectUnknownMembers`. v2 ignores unknown members by default, as `argo.Get`
  does, and matches member names case-sensitively; the twin below is the evidence that the two agree on the cases it covers.
- The two APIs coexist: `argo` stays the lenient, v1-shaped package (duplicate names accepted by `argo.Get`, HTML escapes by
  `argo.Put`), and `jsonv2` is the v2 surface over it. Code that wants v2 imports `jsonv2`.

## Checked against Go

- `tools/ci/jsonv2_check.tin` (CI step): 415 cases, hand-written and generated `Note` and `Point` documents and their mutations,
  each unmarshalled and then marshalled under four option sets, against Go's `encoding/json/v2` (`bench/ref/jsonv2`). Both sides agree
  on which documents decode and on every byte of the output.
- `toolchain/tests/v2/jsonv2.tin`: the options, the escapes, duplicates, unknown members and the unchanged target after a fault.

## Known gaps (differences from encoding/json/v2)

- Nil slices and maps marshal as `[]` and `{}`, matching v2's defaults. `jsonv2` uses argo's separate generated v2 encoders; v1 `argo.Put` continues to write `null`.
- Invalid UTF-8 in a string marshals as `�`; v2 refuses it by default (`argo_utf8`, #104 keeps `argo`'s output).
- Semantic error texts are `argo`'s, not v2's (type mismatch, overflow, `RejectUnknownMembers`: `argo: unknown key "Z" at offset 7`).
  Syntax errors are jsontext's, which match Go. The twin checks whether a document decodes, not the text of its error.
- Not implemented: `FormatNilSliceAsNull`, `FormatNilMapAsNull`, `OmitZeroStructFields`, `StringifyNumbers`, `Deterministic` (argo
  already writes fields in declaration order), `MatchCaseInsensitiveNames`, custom `Marshalers` and `Unmarshalers`, and `[]byte` as
  base64 (not checked against v2).
- The two passes (`argo` then `jsontext`, and `jsontext` then `argo`) cost more than either alone. No performance claim is made.
