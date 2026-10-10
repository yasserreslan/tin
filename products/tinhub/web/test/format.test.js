// The formatting helpers (web/static/js/lib/format.js). Run: node --test products/tinhub/web/test/
import { test } from "node:test";
import assert from "node:assert/strict";
import * as f from "../static/js/lib/format.js";

test("toMs reads seconds, milliseconds and microseconds", () => {
	assert.equal(f.toMs(1791616586), 1791616586000);
	assert.equal(f.toMs(1791616586000), 1791616586000);
	assert.equal(f.toMs(1791616586000000), 1791616586000);
});

test("ago", () => {
	const now = Date.UTC(2026, 9, 10, 12);
	assert.equal(f.ago(now / 1000 - 5, now), "just now");
	assert.match(f.ago(now / 1000 - 3 * 60, now), /^3 minutes ago$/);
	assert.match(f.ago(now / 1000 - 2 * 3600, now), /^2 hours ago$/);
});

test("plural and counts", () => {
	assert.equal(f.plural(1, "change"), "1 change");
	assert.equal(f.plural(2, "change"), "2 changes");
	assert.equal(f.plural(3, "repository", "repositories"), "3 repositories");
	assert.equal(f.bytes(2048), "2 KB");
	assert.equal(f.bytes(0), "0 B");
});

test("ids", () => {
	assert.equal(f.short("8fe7a8b2f896846c11eb45772dd2941368114c0c2fc9fe55ae5b7fa3124eb38b", 8), "8fe7a8b2");
	assert.equal(f.shortChange("lxqqmstqmuuwmwnwsqnryysmmoutyntr"), "lxqqmstqmuuw");
});

test("commit messages", () => {
	assert.equal(f.title("tit: a banner\n\nThe body."), "tit: a banner");
	assert.equal(f.body("tit: a banner\n\nThe body.\nMore."), "The body.\nMore.");
	assert.equal(f.body("only a title"), "");
});

test("languages by path", () => {
	assert.equal(f.language("products/tit/main.tin"), "tin");
	assert.equal(f.language("a.go"), "go");
	assert.equal(f.language("README.md"), "markdown");
	assert.equal(f.basename("a/b/c.tin"), "c.tin");
	assert.equal(f.dirname("a/b/c.tin"), "a/b");
	assert.equal(f.joinPath("a", "", "b"), "a/b");
});

test("percent", () => {
	assert.equal(f.percent(0.123, true), "+12%");
	assert.equal(f.percent(-0.05, true), "-5.0%");
	assert.equal(f.percent(0.5), "50%");
});

test("compareSpec", () => {
	assert.deepEqual(f.compareSpec("main...feature/x", "main"), { from: "main", to: "feature/x" });
	assert.deepEqual(f.compareSpec("v1.0...v1.1", "main"), { from: "v1.0", to: "v1.1" });
	assert.deepEqual(f.compareSpec("feature", "main"), { from: "main", to: "feature" });
	assert.deepEqual(f.compareSpec("", "main"), { from: "main", to: "main" });
	assert.deepEqual(f.compareSpec("...feature", "main"), { from: "main", to: "feature" });
});

test("filterGrouped", () => {
	const items = [{ head: "Branches" }, { label: "main", sub: "default" }, { label: "feature/x" }, { head: "Tags" }, { label: "v1.0" }, { label: "Feature-tag" }];
	assert.deepEqual(f.filterGrouped(items, ""), items);
	assert.deepEqual(
		f.filterGrouped(items, "FEAT").map((x) => x.head || x.label),
		["Branches", "feature/x", "Tags", "Feature-tag"],
	);
	assert.deepEqual(
		f.filterGrouped(items, "default").map((x) => x.head || x.label),
		["Branches", "main"],
	);
	assert.deepEqual(f.filterGrouped(items, "nothing"), []);
});
