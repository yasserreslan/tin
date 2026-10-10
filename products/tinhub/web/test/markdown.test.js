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

test("reference links", () => {
	const n = parse('See [the docs][d], [D][], [d] and ![logo][img], not [x].\n\n[d]: https://e.com/a?b=1&amp;c=2 "Docs"\n[img]: <./logo.png>\n[bad]: javascript:alert(1)\n\n[unsafe][bad]\n\n```\n[x]: https://code.example\n```\n[later]: https://later.example\n\n[later]');
	assert.equal(n.length, 4);
	const links = n[0].c.filter((x) => x.t === "link");
	assert.deepEqual(
		links.map((l) => [plain(l.c), l.href, l.title]),
		[
			["the docs", "https://e.com/a?b=1&c=2", "Docs"],
			["D", "https://e.com/a?b=1&c=2", "Docs"],
			["d", "https://e.com/a?b=1&c=2", "Docs"],
		],
	);
	assert.deepEqual(n[0].c.find((x) => x.t === "img"), { t: "img", src: "./logo.png", alt: "logo", title: "" });
	assert.equal(plain(n[0].c).endsWith("not [x]."), true);
	assert.equal(n[1].c[0].href, "", "a javascript: definition is dropped like any unsafe link");
	assert.equal(n[2].t, "code");
	assert.equal(n[2].v, "[x]: https://code.example", "a definition inside code is code");
	assert.equal(n[3].c[0].href, "https://later.example");
});

test("entities and hard breaks", () => {
	assert.deepEqual(parse("&copy; 2026 &amp; &#169; &#x41; &nosuch; `&amp;`")[0].c, [
		{ t: "text", v: "© 2026 & © A &nosuch; " },
		{ t: "code", v: "&amp;" },
	]);
	assert.equal(toHtml(parse("&lt;script&gt; <b>x</b>")[0].c), "&lt;script&gt; <b>x</b>");
	assert.deepEqual(
		parse("one  \ntwo\\\nthree  ")[0].c.map((x) => x.t),
		["text", "br", "text", "br", "text"],
	);
});
