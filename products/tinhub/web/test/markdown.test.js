// The Markdown reader (web/static/js/lib/markdown.js): structure, and that nothing unsafe survives parsing.
import { test } from "node:test";
import assert from "node:assert/strict";
import { parse, safeUrl, slug, toHtml, plain } from "../static/js/lib/markdown.js";

test("blocks", () => {
	const n = parse("# Title\n\nA *b* **c** `d` ~~e~~\n\n> quote\n\n---\n\n1. one\n2. two\n\n```tin\nfn main() {}\n```\n");
	assert.deepEqual(
		n.map((x) => x.t),
		["h", "p", "quote", "hr", "list", "code"],
	);
	assert.equal(n[0].level, 1);
	assert.deepEqual(
		n[1].c.map((x) => x.t),
		["text", "em", "text", "strong", "text", "code", "text", "del"],
	);
	assert.equal(n[4].ordered, true);
	assert.equal(n[5].lang, "tin");
	assert.equal(n[5].v, "fn main() {}");
});

test("task lists and tables", () => {
	const n = parse("- [x] done\n- [ ] todo\n- plain\n\n| a | b |\n|:--|--:|\n| 1 | 2 |\n");
	assert.deepEqual(
		n[0].items.map((i) => i.task),
		[true, false, null],
	);
	assert.equal(n[1].t, "table");
	assert.deepEqual(n[1].aligns, ["left", "right"]);
	assert.equal(n[1].rows.length, 1);
});

test("unsafe links are dropped", () => {
	assert.equal(safeUrl("javascript:alert(1)"), "");
	assert.equal(safeUrl("JaVaScRiPt:alert(1)"), "");
	assert.equal(safeUrl("data:text/html,x"), "");
	assert.equal(safeUrl("//evil.example.com"), "");
	assert.equal(safeUrl("https://example.com/a"), "https://example.com/a");
	assert.equal(safeUrl("mailto:a@example.com"), "mailto:a@example.com");
	assert.equal(safeUrl("docs/a.md"), "docs/a.md");
	assert.equal(safeUrl("#anchor"), "#anchor");
	const n = parse("[x](javascript:alert(1)) ![y](javascript:x)");
	const link = n[0].c.find((x) => x.t === "link");
	assert.equal(link.href, "");
});

test("inline html is kept as tag tokens, and toHtml escapes text", () => {
	const n = parse("a <b>bold</b> & <script>x</script>");
	const html = toHtml(n[0].c);
	assert.match(html, /<b>bold<\/b>/);
	assert.match(html, /&amp;/);
	assert.doesNotMatch(html, /<script>/);
});

test("slug and plain", () => {
	assert.equal(slug("Hello, World!"), "hello-world");
	assert.equal(slug("tin 1.0 — what's new"), "tin-10--whats-new");
	assert.equal(plain(parse("**a** `b` [c](d)")), "a b c");
});
