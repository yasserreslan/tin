// Route matching (web/static/js/lib/router.js) and the app's route table order.
import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import { compile, match } from "../static/js/lib/router.js";

const table = (patterns) => patterns.map((p) => ({ pattern: p, matcher: compile(p) }));

test("params, optional params and rests", () => {
	const routes = table(["/", "/:owner", "/:owner/:repo", "/:owner/:repo/tree/:rev/*path", "/:owner/:repo/change/:id/:tab?"]);
	assert.equal(match(routes, "/").route.pattern, "/");
	assert.deepEqual(match(routes, "/ada").params, { owner: "ada" });
	assert.deepEqual(match(routes, "/ada/tin/").params, { owner: "ada", repo: "tin" });
	assert.deepEqual(match(routes, "/ada/tin/tree/main/products/tit").params, { owner: "ada", repo: "tin", rev: "main", path: "products/tit" });
	assert.deepEqual(match(routes, "/ada/tin/tree/main").params, { owner: "ada", repo: "tin", rev: "main", path: "" });
	assert.deepEqual(match(routes, "/ada/tin/change/abc").params, { owner: "ada", repo: "tin", id: "abc", tab: "" });
	assert.deepEqual(match(routes, "/ada/tin/change/abc/files").params, { owner: "ada", repo: "tin", id: "abc", tab: "files" });
	assert.equal(match(routes, "/a/b/c/d/e/f/g"), null);
});

test("percent-encoded segments are decoded, bad ones kept", () => {
	const routes = table(["/:owner/:repo/blob/:rev/*path"]);
	assert.equal(match(routes, "/a/b/blob/feature%2Fx/a%20b.md").params.rev, "feature/x");
	assert.equal(match(routes, "/a/b/blob/main/a%20b.md").params.path, "a b.md");
	assert.equal(match(routes, "/a/b/blob/main/%E0%A4%A").params.path, "%E0%A4%A");
});

test("every page route in main.js resolves to its page, and fixed paths win over owners", () => {
	const src = fs.readFileSync(new URL("../static/js/main.js", import.meta.url), "utf8");
	const patterns = [...src.matchAll(/\["(\/[^"]*)", page\("\.\/pages\/([a-z]+)\.js"\)/g)].map((m) => ({ pattern: m[1], page: m[2] }));
	assert.ok(patterns.length >= 30);
	const routes = patterns.map((p) => ({ ...p, matcher: compile(p.pattern) }));
	const pageOf = (path) => match(routes, path)?.route.page;
	assert.equal(pageOf("/explore"), "explore");
	assert.equal(pageOf("/settings/keys"), "settings");
	assert.equal(pageOf("/new/org"), "neworg");
	assert.equal(pageOf("/ada"), "owner");
	assert.equal(pageOf("/ada/tin"), "code");
	assert.equal(pageOf("/ada/tin/blob/main/README.md"), "code");
	assert.equal(pageOf("/ada/tin/change/abc/files"), "review");
	assert.equal(pageOf("/ada/tin/settings/webhooks"), "reposettings");
	assert.equal(pageOf("/ada/tin/bench/fib"), "bench");
	assert.equal(pageOf("/ada/tin/releases/v1.0.0"), "releases");
	for (const p of new Set(patterns.map((x) => x.page))) assert.ok(fs.existsSync(new URL(`../static/js/pages/${p}.js`, import.meta.url)), p);
});

test("the reserved owner names cover the app's first segments", () => {
	const src = fs.readFileSync(new URL("../static/js/main.js", import.meta.url), "utf8");
	const tin = fs.readFileSync(new URL("../../accounts/accounts.tin", import.meta.url), "utf8");
	const reserved = /const reservedOwners = "([^"]*)"/.exec(tin)[1].trim().split(/\s+/);
	const firsts = new Set([...src.matchAll(/\["\/([a-z]+)/g)].map((m) => m[1]));
	for (const f of firsts) assert.ok(reserved.includes(f), `${f} is a page, so no one may own it`);
});
