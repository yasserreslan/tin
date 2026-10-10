// tinhub's web UI (design/tinhub.md §15): boots the frame, reads the signed-in user and routes every page. Pages are
// modules loaded on first use; each exports render(ctx) and builds its content into ctx.main.

import { h, clear } from "./lib/dom.js";
import { define, start, navigate } from "./lib/router.js";
import * as store from "./lib/store.js";
import * as api from "./lib/api.js";
import { header, footer, keys } from "./ui/layout.js";
import { errorBox, loading } from "./ui/kit.js";

const page = (path) => () => import(path);

const routes = define([
	["/", page("./pages/home.js")],
	["/login", page("./pages/signin.js")],
	["/join", page("./pages/join.js")],
	["/explore", page("./pages/explore.js")],
	["/search", page("./pages/search.js")],
	["/new", page("./pages/newrepo.js")],
	["/new/org", page("./pages/neworg.js")],
	["/settings/:section?", page("./pages/settings.js")],
	["/admin/:section?", page("./pages/admin.js")],
	["/:owner", page("./pages/owner.js")],
	["/:owner/:repo", page("./pages/code.js")],
	["/:owner/:repo/tree/:rev/*path", page("./pages/code.js")],
	["/:owner/:repo/blob/:rev/*path", page("./pages/code.js"), { blob: true }],
	["/:owner/:repo/commits/:rev/*path", page("./pages/commits.js")],
	["/:owner/:repo/commits", page("./pages/commits.js")],
	["/:owner/:repo/commit/:id", page("./pages/commit.js")],
	["/:owner/:repo/refs", page("./pages/refs.js")],
	["/:owner/:repo/changes", page("./pages/changes.js")],
	["/:owner/:repo/stacks/:user", page("./pages/stacks.js")],
	["/:owner/:repo/stacks", page("./pages/stacks.js")],
	["/:owner/:repo/reviews", page("./pages/reviews.js")],
	["/:owner/:repo/change/:id/:tab?", page("./pages/review.js")],
	["/:owner/:repo/releases", page("./pages/releases.js")],
	["/:owner/:repo/releases/*tag", page("./pages/releases.js")],
	["/:owner/:repo/bench", page("./pages/bench.js")],
	["/:owner/:repo/bench/*name", page("./pages/bench.js")],
	["/:owner/:repo/replay", page("./pages/replay.js")],
	["/:owner/:repo/replay/:group", page("./pages/replay.js")],
	["/:owner/:repo/activity", page("./pages/activity.js")],
	["/:owner/:repo/settings/:section?", page("./pages/reposettings.js")],
]);

const main = h("main.main", { id: "main" });
let current = { cleanups: [], token: 0, mod: null };

function setTitle(...parts) {
	document.title = [...parts.filter(Boolean), "tinhub"].join(" · ");
}

async function route(r) {
	if (r.hashOnly && current.mod && current.mod.onHash) {
		current.mod.onHash(r.hash);
		return;
	}
	for (const fn of current.cleanups) {
		try {
			fn();
		} catch {}
	}
	const token = ++current.token;
	current = { cleanups: [], token, mod: null };
	if (!r.route) {
		notFound();
		return;
	}
	const slow = setTimeout(() => {
		if (current.token === token) main.replaceChildren(loading());
	}, 160);
	let mod;
	try {
		mod = await r.route.load();
	} catch (err) {
		clearTimeout(slow);
		main.replaceChildren(errorBox(err, () => location.reload()));
		return;
	}
	if (current.token !== token) return;
	current.mod = mod;
	const ctx = {
		params: r.params,
		query: r.query,
		hash: r.hash,
		route: r.route,
		main,
		title: setTitle,
		alive: () => current.token === token,
		cleanup: (fn) => current.cleanups.push(fn),
		user: store.session.user,
		notFound,
	};
	try {
		await mod.render(ctx);
	} catch (err) {
		if (current.token !== token) return;
		console.error(err);
		if (err && err.status === 401) {
			navigate("/login?next=" + encodeURIComponent(location.pathname + location.search), { replace: true });
			return;
		}
		main.replaceChildren(h("div.container.page", {}, errorBox(err, () => route(r))));
	} finally {
		clearTimeout(slow);
	}
	if (current.token === token && r.scroll !== false) {
		if (r.hash) {
			const el = document.getElementById(r.hash);
			if (el) el.scrollIntoView({ block: "center" });
		} else window.scrollTo(0, 0);
	}
}

export function notFound() {
	setTitle("Not found");
	main.replaceChildren(
		h(
			"div.container.page.notfound",
			{},
			h("div.big", {}, "404"),
			h("h2", { style: { margin: "16px 0 8px" } }, "This page isn't here"),
			h("p.muted", {}, "It may be private, moved, or never have existed."),
			h("div.row", { style: { "justify-content": "center", "margin-top": "24px" } }, h("a.btn.primary", { href: "/" }, "Go home"), h("a.btn", { href: "/explore" }, "Explore")),
		),
	);
}

async function boot() {
	store.applyTheme();
	const root = document.getElementById("app");
	await store.loadUser();
	let warned = false;
	api.setUnauthorized(() => {
		if (store.session.user && !warned) {
			warned = true;
			store.setUser(null);
		}
	});
	clear(root);
	root.append(h("div.app", {}, header(), main, footer()));
	keys();
	start(routes, (r) => route(r));
	window.matchMedia("(prefers-color-scheme: dark)").addEventListener?.("change", () => store.applyTheme());
}

boot();
