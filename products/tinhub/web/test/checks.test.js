// Replay checks on the group page (web/static/js/ui/checks.js, lib/verdicts.js). Run: node --test products/tinhub/web/test/
import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import { worst } from "../static/js/lib/verdicts.js";

test("an element with the hidden attribute is hidden whatever display its class sets", () => {
	// a check's drawer is a flex column toggled with `hidden`; without this rule it opened and never closed
	const css = fs.readFileSync(new URL("../static/css/app.css", import.meta.url), "utf8");
	assert.match(css, /^\[hidden\]\s*\{\s*display:\s*none\s*!important;\s*\}/m);
	assert.match(css, /\.rcheck-detail\s*\{[^}]*display:\s*flex/);
});

test("a check's outcome is its worst verdict", () => {
	assert.equal(worst([{ verdict: "passed", count: 3 }]), "passed");
	assert.equal(worst([{ verdict: "passed", count: 3 }, { verdict: "diverged", count: 1 }]), "diverged");
	assert.equal(worst([{ verdict: "diverged", count: 2 }, { verdict: "panicked", count: 1 }, { verdict: "failing", count: 1 }]), "panicked");
	assert.equal(worst([{ verdict: "failing", count: 2 }, { verdict: "timeout", count: 1 }]), "failing");
	assert.equal(worst([{ verdict: "panicked", count: 0 }, { verdict: "passed", count: 1 }]), "passed");
	assert.equal(worst([]), "");
	assert.equal(worst(undefined), "");
});
