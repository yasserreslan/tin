// The router: patterns like "/:owner/:repo/tree/:rev/*path", matched in order. Matching is pure (tested by
// web/test/router.test.js); start() binds it to the browser's history and intercepts same-origin link clicks.

// compile turns a pattern into a matcher.
export function compile(pattern) {
	const keys = [];
	const parts = pattern.split("/").filter(Boolean);
	let re = "^";
	for (const p of parts) {
		if (p.startsWith(":")) {
			const opt = p.endsWith("?");
			const name = opt ? p.slice(1, -1) : p.slice(1);
			keys.push(name);
			re += opt ? "(?:/([^/]+))?" : "/([^/]+)";
		} else if (p.startsWith("*")) {
			keys.push(p.slice(1) || "rest");
			re += "(?:/(.*))?";
		} else {
			re += "/" + p.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
		}
	}
	re += "/?$";
	return { pattern, keys, re: new RegExp(re) };
}

// match finds the first route whose pattern matches path: {route, params} or null.
export function match(routes, path) {
	for (const r of routes) {
		const m = r.matcher.re.exec(path === "" ? "/" : path);
		if (!m) continue;
		const params = {};
		r.matcher.keys.forEach((k, i) => {
			const v = m[i + 1];
			params[k] = v === undefined ? "" : safeDecode(v);
		});
		return { route: r, params };
	}
	return null;
}

function safeDecode(v) {
	try {
		return v.split("/").map(decodeURIComponent).join("/");
	} catch {
		return v;
	}
}

// define makes the route table from [pattern, handler, extra] entries.
export function define(entries) {
	return entries.map(([pattern, load, extra]) => ({ pattern, load, matcher: compile(pattern), ...(extra || {}) }));
}

let table = [];
let handler = null;
let current = "";

// start routes the current URL and every later navigation through onRoute({route, params, path, query, hash}).
export function start(routes, onRoute) {
	table = routes;
	handler = onRoute;
	window.addEventListener("popstate", () => dispatch(false));
	document.addEventListener("click", (e) => {
		if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
		const a = e.target.closest && e.target.closest("a");
		if (!a || a.target === "_blank" || a.hasAttribute("download") || a.dataset.native !== undefined) return;
		const href = a.getAttribute("href");
		if (!href || href.startsWith("#") || href.startsWith("mailto:")) return;
		const url = new URL(a.href, location.href);
		if (url.origin !== location.origin) return;
		if (url.pathname.startsWith("/api/") || url.pathname.startsWith("/-/") || url.pathname.startsWith("/tit/")) return;
		e.preventDefault();
		navigate(url.pathname + url.search + url.hash);
	});
	dispatch(false);
}

// navigate goes to a URL inside the app; replace keeps the history entry.
export function navigate(url, { replace = false } = {}) {
	const u = new URL(url, location.href);
	const same = u.pathname + u.search === location.pathname + location.search;
	if (replace) history.replaceState(null, "", u.pathname + u.search + u.hash);
	else if (!same || u.hash !== location.hash) history.pushState(null, "", u.pathname + u.search + u.hash);
	dispatch(!same);
}

// loginHref is the sign-in page's address, coming back here afterwards. On the sign-in page itself it keeps the
// address it was given, so a second click does not send the visitor back to sign-in.
export function loginHref() {
	if (location.pathname === "/login") return "/login" + location.search;
	return "/login?next=" + encodeURIComponent(location.pathname + location.search + location.hash);
}

// setQuery changes the URL's query without a new history entry and without routing again.
export function setQuery(params, { push = false } = {}) {
	const q = new URLSearchParams(location.search);
	for (const [k, v] of Object.entries(params)) {
		if (v === undefined || v === null || v === "" || v === false) q.delete(k);
		else q.set(k, String(v));
	}
	const s = q.toString();
	const url = location.pathname + (s ? "?" + s : "") + location.hash;
	if (push) history.pushState(null, "", url);
	else history.replaceState(null, "", url);
}

// reload routes the current URL again.
export function reload() {
	dispatch(false, true);
}

function dispatch(scroll, force = false) {
	const path = location.pathname;
	const key = path + location.search;
	const hashOnly = !force && key === current;
	current = key;
	const m = match(table, path);
	const queryParams = Object.fromEntries(new URLSearchParams(location.search));
	handler({ route: m ? m.route : null, params: m ? m.params : {}, path, query: queryParams, hash: decodeURIComponent(location.hash.slice(1)), scroll, hashOnly });
}
