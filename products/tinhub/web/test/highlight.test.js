// The highlighter and the Tin outline (web/static/js/lib/highlight.js).
import { test } from "node:test";
import assert from "node:assert/strict";
import { lines, outline } from "../static/js/lib/highlight.js";

const classes = (rows) => rows.flat().filter(([c]) => c).map(([c, t]) => `${c}:${t}`);

test("tin tokens", () => {
	const got = classes(lines('fn main() {\n\tlet x = "a{b}" // c\n}', "tin"));
	assert.ok(got.includes("tok-kw:fn"));
	assert.ok(got.includes("tok-fn:main"));
	assert.ok(got.includes("tok-kw:let"));
	assert.ok(got.includes("tok-interp:{b}"));
	assert.ok(got.includes("tok-com:// c"));
});

test("every line is kept, text unchanged", () => {
	const src = "package x\n\n// a\n// b\nfn f() int {\n\treturn 1\n}\n";
	const rows = lines(src, "tin");
	assert.equal(rows.length, 7);
	assert.equal(rows.map((r) => r.map(([, t]) => t).join("")).join("\n") + "\n", src);
	assert.ok(classes(rows).some((x) => x.startsWith("tok-com:")));
});

test("unknown languages are plain", () => {
	assert.deepEqual(lines("a\nb", "nothing"), [[["", "a"]], [["", "b"]]]);
});

test("outline", () => {
	const o = outline("fn main() {\n}\ntype A struct {\n}\nfn (a A) Go() {}\nconst B = 1\nlet s = `\nfn notReal() {}\n`\n");
	assert.deepEqual(
		o.map((d) => `${d.kind} ${d.recv ? d.recv + "." : ""}${d.name} ${d.line}`),
		["fn main 1", "type A 3", "method A.Go 5", "const B 6", "let s 7"],
	);
});
