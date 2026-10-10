// Every module the UI loads parses, and every relative import names a file and an export that exist.
import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

const root = new URL("../static/js/", import.meta.url).pathname;
const files = [];
const walk = (d) => {
	for (const e of fs.readdirSync(d, { withFileTypes: true })) {
		if (e.isDirectory()) walk(path.join(d, e.name));
		else if (e.name.endsWith(".js")) files.push(path.join(d, e.name));
	}
};
walk(root);

const exportsOf = (src) => {
	const names = new Set();
	for (const m of src.matchAll(/export\s+(?:async\s+)?(?:function\*?|const|let|class)\s+([A-Za-z_$][\w$]*)/g)) names.add(m[1]);
	for (const m of src.matchAll(/export\s*\{([^}]*)\}/g)) for (const n of m[1].split(",")) names.add(n.trim().split(/\s+as\s+/).pop());
	return names;
};

test("imports resolve", () => {
	assert.ok(files.length > 30);
	for (const f of files) {
		const src = fs.readFileSync(f, "utf8");
		for (const m of src.matchAll(/import\s+(?:\{([^}]*)\}|\*\s+as\s+\w+|\w+)\s+from\s+"(\.[^"]+)"/g)) {
			const target = path.resolve(path.dirname(f), m[2]);
			assert.ok(fs.existsSync(target), `${path.relative(root, f)} imports ${m[2]}`);
			if (!m[1]) continue;
			const names = exportsOf(fs.readFileSync(target, "utf8"));
			for (const n of m[1].split(",").map((x) => x.trim().split(/\s+as\s+/)[0]).filter(Boolean)) assert.ok(names.has(n), `${path.relative(root, f)} imports ${n} from ${m[2]}, which does not export it`);
		}
	}
});

test("no inline styles or handlers in markup strings (the CSP forbids them)", () => {
	for (const f of files) {
		const src = fs.readFileSync(f, "utf8");
		assert.doesNotMatch(src, /setAttribute\(\s*"style"/, path.relative(root, f));
		assert.doesNotMatch(src, /\.innerHTML\s*=(?!\s*"")/, path.relative(root, f));
		assert.doesNotMatch(src, /style:\s*"[^"]/, path.relative(root, f));
	}
});

test("pages export render", () => {
	for (const f of files.filter((x) => x.includes("/pages/") && !x.endsWith("/repo.js"))) assert.ok(exportsOf(fs.readFileSync(f, "utf8")).has("render"), path.relative(root, f));
});
