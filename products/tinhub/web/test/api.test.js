// The API client's message tidying (web/static/js/lib/api.js). Run: node --test products/tinhub/web/test/
import { test } from "node:test";
import assert from "node:assert/strict";
import { tidy } from "../static/js/lib/api.js";

test("tidy drops the fault kinds a message ends with", () => {
	assert.equal(tidy("ada/tin exists: already exists"), "ada/tin exists");
	assert.equal(tidy("the secret must be 16 to 200 bytes: invalid"), "the secret must be 16 to 200 bytes");
	assert.equal(tidy("127.0.0.1 is not a public address: webhook URL: invalid"), "127.0.0.1 is not a public address");
	assert.equal(tidy("no webhook 5: not found"), "no webhook 5");
});

test("tidy keeps a message that is only a kind, or has none", () => {
	assert.equal(tidy("invalid"), "invalid");
	assert.equal(tidy(": invalid"), ": invalid");
	assert.equal(tidy("no user nobody-here"), "no user nobody-here");
	assert.equal(tidy("deadline exceeded"), "deadline exceeded");
	assert.equal(tidy(undefined), "");
});
