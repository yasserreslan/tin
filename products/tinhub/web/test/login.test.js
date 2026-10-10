// The sign-in link's return address (web/static/js/lib/router.js loginHref).
import { test } from "node:test";
import assert from "node:assert/strict";

globalThis.location = new URL("http://tinhub.test/");
const { loginHref } = await import("../static/js/lib/router.js");
const at = (url) => (globalThis.location = new URL(url, "http://tinhub.test"));

test("loginHref comes back to the page it is used on, with its query and anchor", () => {
	at("/explore");
	assert.equal(loginHref(), "/login?next=%2Fexplore");
	at("/ada/tin/change/abc?v=2#c-7");
	assert.equal(loginHref(), "/login?next=%2Fada%2Ftin%2Fchange%2Fabc%3Fv%3D2%23c-7");
});

test("on the sign-in page loginHref keeps the address it was given", () => {
	at("/login?next=%2Fada%2Ftin");
	assert.equal(loginHref(), "/login?next=%2Fada%2Ftin");
	at("/login");
	assert.equal(loginHref(), "/login");
});
