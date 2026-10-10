// Page-wide state: the signed-in user, the site's build and URL (from the shell's meta tags), the theme, and small
// per-browser preferences kept in localStorage.

import * as api from "./api.js";

function meta(name) {
	const el = document.querySelector(`meta[name="${name}"]`);
	return el ? el.getAttribute("content") || "" : "";
}

export const site = {
	build: meta("tinhub-build"),
	url: (meta("tinhub-public-url") || location.origin).replace(/\/$/, ""),
	version: meta("tinhub-version"),
	host: "",
};
site.host = site.url.replace(/^https?:\/\//, "");

export const session = { user: null, loaded: false };

const watchers = new Set();

// onUser calls fn whenever the signed-in user changes.
export function onUser(fn) {
	watchers.add(fn);
	return () => watchers.delete(fn);
}

function changed() {
	for (const fn of watchers) fn(session.user);
}

// loadUser reads the signed-in user (null for nobody).
export async function loadUser() {
	try {
		session.user = await api.get("/user");
	} catch (err) {
		session.user = null;
		if (err.status && err.status !== 401 && err.status !== 404) console.warn("tinhub: /api/v1/user:", err.message);
	}
	session.loaded = true;
	changed();
	return session.user;
}

export function setUser(u) {
	session.user = u;
	changed();
}

export async function signOut() {
	try {
		await fetch("/tit/v1/logout", { method: "POST", credentials: "same-origin" });
	} catch {}
	session.user = null;
	changed();
}

// pref reads and writes a per-browser preference.
export function pref(key, fallback) {
	try {
		const v = localStorage.getItem("tinhub." + key);
		return v === null ? fallback : JSON.parse(v);
	} catch {
		return fallback;
	}
}

export function setPref(key, value) {
	try {
		localStorage.setItem("tinhub." + key, JSON.stringify(value));
	} catch {}
}

// theme is "system", "light" or "dark".
export function theme() {
	return pref("theme", "system");
}

export function applyTheme(t = theme()) {
	if (t === "light" || t === "dark") document.documentElement.dataset.theme = t;
	else delete document.documentElement.dataset.theme;
}

export function setTheme(t) {
	setPref("theme", t);
	applyTheme(t);
}

// recent remembers the repositories the user opened, newest first, for the palette and the dashboard.
export function recent() {
	return pref("recent", []);
}

export function visit(full) {
	const list = recent().filter((r) => r !== full);
	list.unshift(full);
	setPref("recent", list.slice(0, 12));
}
