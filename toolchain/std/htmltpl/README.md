# htmltpl

Go's `html/template` for Tin: a template is parsed once, and every action is escaped by the context it sits in, so a
value from outside the program cannot leave its place in the markup. The contextual engine is stencil's
(`stencil.ParseContextual`, in `toolchain/std/stencil/ctx.tin` and `ctx_escape.tin`); this package is the Go-shaped API
over it. `stencil.ParseHTML` keeps its narrower mode, which refuses the contexts this one supports.

## API

| Go | Tin |
| --- | --- |
| `template.New(name)` | `htmltpl.New(name) Template` |
| `t.Funcs(m)` | `t.Funcs(m map[str]fn([]stencil.Value) !stencil.Value)` (before `Parse`) |
| `t.Parse(text)` | `t.Parse(text) !` (replaces the body) |
| `t.Execute(w, data)` | `t.Execute(w mut twine.Builder, data stencil.Value) !` |
| (string result) | `t.ExecuteString(data) !str` |
| `t.Name()` | `t.Name() str` |
| `template.HTML(s)` | `htmltpl.HTML(s) stencil.Value` |
| `template.JS(s)` | `htmltpl.JS(s) stencil.Value` |
| `template.JSStr(s)` | `htmltpl.JSStr(s) stencil.Value` |
| `template.CSS(s)` | `htmltpl.CSS(s) stencil.Value` |
| `template.URL(s)` | `htmltpl.URL(s) stencil.Value` |

Data is a `stencil.Value` (strings, integers, floats, booleans, lists, maps and none), built with `stencil.Str` and its
siblings. A trusted value is built with `htmltpl.HTML` and the others; template data from outside the program is never
trusted, because only these constructors set the trust kind.

## Contexts

The escaper of an action follows the markup before it, as Go's escaper does:

- element text: `<` in static text that is not markup is written `&lt;`; the value is HTML-escaped (a trusted HTML value passes);
- comments: static comment text is dropped and an action inside one gives nothing;
- RCDATA (`<title>`, `<textarea>`): text escaping, and trusted HTML escapes its `<` and `>` but not `&`;
- attribute values, quoted or unquoted (an empty unquoted value is `ZgotmplZ`); trusted HTML in an attribute has its tags stripped, as Go strips them;
- URL attributes (`href`, `src`, `action`, ...): at the start the scheme is filtered (`#ZgotmplZ`), then the value is
  normalized, and in a query or fragment it is percent-encoded; a trusted URL is only normalized;
- `style` and `on*` attributes: the value is a CSS or JavaScript value, escaped as the attribute's value;
- `<script>`: a JavaScript value is JSON (padded with spaces next to an identifier byte, as Go pads it), a string is
  escaped for a JavaScript string literal; `//` and `/* */` comments are dropped, leaving a space or a newline;
- `<style>`: a CSS value is filtered (`ZgotmplZ` where it could leave its declaration), a string is CSS-escaped, a
  `url(...)` is a URL, and comments are dropped.

The twin in `tools/ci/htmltpl_check.tin` checks 10,286 template and value pairs against Go's `html/template`
(`bench/ref/htmltpl`): every context above with a `<script>` payload, the trusted types, numbers, booleans and nil.

## Known gaps

These are refused at parse time with a message that names the reason (stencil's errors), where Go accepts them. Nothing
is escaped by a guess.

- an action in a tag or an attribute name, in a `srcset` value, in a regular expression, or in a `meta` tag;
- a template literal (a backquote) in a `<script>`, and `<script`, `</script` or `<!--` inside a JavaScript string;
- the predefined escapers `html` and `urlquery` in an action (`js` is allowed, as a function);
- a `{{template}}` call outside element text, and an `if`, `range` or `with` block that ends in another context;
- a context that ends inside a tag, an attribute, a string or a script.

Differences in shape, not in output:

- `Parse` replaces the body and does not accumulate `{{define}}` into a set as Go's `Parse` does;
- Go reports most context errors at `Execute`; here they are `Parse` errors;
- `Funcs` takes functions over `stencil.Value`, not `any`, and the data is a `stencil.Value`, not a Go value;
- `template.Srcset` and `template.HTMLAttr` are not provided (no context here takes a value of that kind);
- the Go-named escaper functions (`HTMLEscapeString`, `JSEscapeString`, `URLQueryEscaper`) are not exported yet.

## Tests

- `toolchain/tests/v2/htmltpl.tin`: the API, every context, the trusted types and the refusals (`htmltpl.out`, each
  valid line checked against Go's output for the same template and data).
- `tools/ci/htmltpl_check.tin`: the 10,286-pair twin against `html/template`.
