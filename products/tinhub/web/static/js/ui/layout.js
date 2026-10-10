// The page frame: the top bar (brand, search, nav, the user's menu), the footer, and the command palette.

import { h, clear, debounce } from "../lib/dom.js";
import { icon, mark } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as store from "../lib/store.js";
import * as live from "../lib/live.js";
import { navigate } from "../lib/router.js";
import { avatar, btn, menu, loginHref } from "./kit.js";

export function header() {
	const end = h("div.topbar-end");
	const nav = h("nav.topnav");
	const search = h("button.search-trigger", { type: "button", onclick: () => palette() }, icon("search", "sm"), h("span.label", {}, "Search or jump to…"), h("kbd", {}, "/"));
	const dot = h("span.live-dot", { title: "Live updates" });
	live.onStatus((on) => {
		dot.classList.toggle("on", on);
		dot.classList.toggle("hidden", !on);
		dot.title = on ? "Live updates are on" : "Live updates are off";
	});
	const draw = (u) => {
		clear(nav);
		clear(end);
		nav.append(h("a", { href: "/explore" }, "Explore"), u ? h("a", { href: "/" + u.name + "?tab=repos" }, "Your repositories") : null, u && u.site_admin ? h("a", { href: "/admin/invites" }, "Admin") : null);
		if (u) {
			const plus = btn("", { icon: "plus", ghost: true, title: "Create" });
			plus.onclick = () =>
				menu(
					plus,
					[
						{ label: "New repository", icon: "repo", href: "/new" },
						{ label: "New organization", icon: "org", href: "/new/org" },
						u.site_admin ? { label: "Invite someone", icon: "mail", href: "/admin/invites" } : null,
					].filter(Boolean),
					{ align: "right" },
				);
			const me = h("button.btn.ghost", { type: "button", style: { padding: "0 4px", gap: "4px" }, "aria-label": "Your menu" }, avatar(u.display || u.name, "", {}), icon("chevronDown", "sm"));
			me.onclick = () => userMenu(me, u);
			end.append(dot, plus, me);
		} else {
			// The header outlives the page, so the link takes the address it comes back to when it is used.
			const signIn = btn("Sign in", { icon: "login", primary: true, href: loginHref() });
			const fresh = () => signIn.setAttribute("href", loginHref());
			for (const ev of ["pointerdown", "focus", "click"]) signIn.addEventListener(ev, fresh);
			end.append(themeButton(), signIn);
		}
	};
	store.onUser(draw);
	draw(store.session.user);
	return h("header.topbar", {}, h("div.container.topbar-inner", {}, h("a.brand", { href: "/", "aria-label": "tinhub home" }, mark(26), "tinhub"), search, nav, end));
}

function themeButton() {
	const t = store.theme();
	const b = btn("", { ghost: true, icon: t === "dark" ? "moon" : t === "light" ? "sun" : "monitor", title: "Theme" });
	b.onclick = () => themeMenu(b);
	return b;
}

function themeMenu(anchor) {
	const t = store.theme();
	menu(
		anchor,
		[
			{ head: "Theme" },
			{ label: "System", icon: "monitor", checked: t === "system", onclick: () => store.setTheme("system") },
			{ label: "Light", icon: "sun", checked: t === "light", onclick: () => store.setTheme("light") },
			{ label: "Dark", icon: "moon", checked: t === "dark", onclick: () => store.setTheme("dark") },
		],
		{ align: "right" },
	);
}

function userMenu(anchor, u) {
	const t = store.theme();
	menu(
		anchor,
		[
			{ node: h("div", { style: { padding: "8px 10px" } }, h("div.strong", {}, u.display || u.name), h("div.muted.small", {}, u.email)) },
			{ sep: true },
			{ label: "Your profile", icon: "user", href: "/" + u.name },
			{ label: "Your repositories", icon: "repo", href: "/" + u.name + "?tab=repos" },
			...(u.orgs || []).map((o) => ({ label: o.display || o.name, icon: "org", href: "/" + o.name, hint: o.role })),
			{ sep: true },
			{ label: "Settings", icon: "settings", href: "/settings/profile" },
			{ label: "Keys", icon: "key", href: "/settings/keys" },
			u.site_admin ? { label: "Site admin", icon: "shield", href: "/admin/invites" } : null,
			{ sep: true },
			{ head: "Theme" },
			{ label: "System", icon: "monitor", checked: t === "system", onclick: () => store.setTheme("system") },
			{ label: "Light", icon: "sun", checked: t === "light", onclick: () => store.setTheme("light") },
			{ label: "Dark", icon: "moon", checked: t === "dark", onclick: () => store.setTheme("dark") },
			{ sep: true },
			{
				label: "Sign out",
				icon: "logout",
				onclick: async () => {
					await store.signOut();
					navigate("/");
				},
			},
		].filter(Boolean),
		{ align: "right", width: 260 },
	);
}

export function footer() {
	return h(
		"footer.footer",
		{},
		h(
			"div.container.row",
			{},
			h("span.row", {}, mark(18), h("span", {}, "tinhub ", store.site.version ? h("span.mono", {}, store.site.version) : null)),
			h("span.row", { style: { gap: "16px" } }, h("a", { href: "/explore" }, "Explore"), h("a", { href: "/search" }, "Search"), h("a", { href: "/api/v1/user", "data-native": "" }, "API"), h("a", { href: "/healthz", "data-native": "" }, "Status")),
		),
	);
}

// ---- the command palette ----

let paletteOpen = null;

// commands are the palette's fixed entries; pages may add their own with addCommands while they are shown.
const extra = new Set();

export function addCommands(list) {
	extra.add(list);
	return () => extra.delete(list);
}

function baseCommands() {
	const u = store.session.user;
	const out = [
		{ label: "Home", icon: "home", href: "/", group: "Go to" },
		{ label: "Explore repositories", icon: "globe", href: "/explore", group: "Go to" },
		{ label: "Search symbols", icon: "search", href: "/search", group: "Go to" },
	];
	if (u) {
		out.push(
			{ label: "New repository", icon: "plus", href: "/new", group: "Create" },
			{ label: "New organization", icon: "org", href: "/new/org", group: "Create" },
			{ label: "Your profile", icon: "user", href: "/" + u.name, group: "Go to" },
			{ label: "Settings: profile", icon: "settings", href: "/settings/profile", group: "Settings" },
			{ label: "Settings: keys", icon: "key", href: "/settings/keys", group: "Settings" },
			{ label: "Settings: following", icon: "bell", href: "/settings/following", group: "Settings" },
		);
		if (u.site_admin) out.push({ label: "Admin: invites", icon: "mail", href: "/admin/invites", group: "Settings" }, { label: "Admin: audit log", icon: "scroll", href: "/admin/audit", group: "Settings" });
	} else out.push({ label: "Sign in", icon: "login", href: "/login", group: "Go to" });
	out.push(
		{ label: "Theme: system", icon: "monitor", run: () => store.setTheme("system"), group: "Theme" },
		{ label: "Theme: light", icon: "sun", run: () => store.setTheme("light"), group: "Theme" },
		{ label: "Theme: dark", icon: "moon", run: () => store.setTheme("dark"), group: "Theme" },
	);
	for (const list of extra) out.push(...list);
	return out;
}

// The repository a page shows, so the palette can find its files: { owner, name, rev }, null when none. A page sets it
// with setRepo, which gives the cleanup that unsets it.
let repoShown = null;
const fileLists = new Map();

export function setRepo(r) {
	repoShown = r;
	return () => {
		if (repoShown === r) repoShown = null;
	};
}

// filesOf is every file path of the repository at its revision, fetched once (the compare from nothing lists them).
function filesOf(r) {
	const key = `${r.owner}/${r.name}@${r.rev}`;
	if (!fileLists.has(key)) {
		if (fileLists.size >= 4) fileLists.delete(fileLists.keys().next().value);
		const list = api
			.get(api.R(r.owner, r.name)("/compare"), { to: r.rev })
			.then((x) => (x.files || []).map((f) => f.path))
			.catch(() => {
				fileLists.delete(key);
				return [];
			});
		fileLists.set(key, list);
	}
	return fileLists.get(key);
}

// pathScore ranks a path for q: its file name first, then the whole path, then q's letters in order in the file name
// (in the whole path when q names a directory).
function pathScore(path, q) {
	const p = path.toLowerCase();
	const base = p.slice(p.lastIndexOf("/") + 1);
	if (base.startsWith(q)) return 5;
	if (base.includes(q)) return 4;
	if (p.includes(q)) return 3;
	let i = 0;
	for (const c of q.includes("/") ? p : base) if (c === q[i]) i++;
	return i === q.length ? 1 : 0;
}

function score(label, q) {
	const l = label.toLowerCase();
	if (!q) return 1;
	if (l.startsWith(q)) return 3;
	if (l.includes(q)) return 2;
	let i = 0;
	for (const c of l) if (c === q[i]) i++;
	return i === q.length ? 1 : 0;
}

export function palette(initial = "") {
	if (paletteOpen) return;
	const input = h("input", { placeholder: repoShown ? "Jump to a file, repository, page or command…  (type # for symbols)" : "Jump to a repository, page or command…  (type # for symbols)", "aria-label": "Search", autocomplete: "off", spellcheck: "false" });
	input.value = initial;
	const list = h("div.palette-list", { role: "listbox" });
	let items = [];
	let focus = 0;
	let remote = [];
	let seq = 0;
	const prev = document.activeElement;
	const draw = () => {
		const q = input.value.trim().toLowerCase();
		const symbolMode = q.startsWith("#");
		const recent = store.recent().map((r) => ({ label: r, icon: "repo", href: "/" + r, group: "Recent" }));
		let local = symbolMode ? [] : [...recent, ...baseCommands()].map((c) => ({ ...c, s: score(c.label, q) })).filter((c) => c.s > 0);
		local.sort((a, b) => b.s - a.s);
		const seen = new Set();
		items = [...remote, ...local].filter((c) => {
			const k = c.href || c.label;
			if (seen.has(k)) return false;
			seen.add(k);
			return true;
		});
		if (q && !symbolMode) items.push({ label: `Search symbols for “${input.value.trim()}”`, icon: "search", href: "/search?q=" + encodeURIComponent(input.value.trim()), group: "Search" });
		items = items.slice(0, 40);
		focus = Math.min(focus, Math.max(0, items.length - 1));
		clear(list);
		let group = "";
		items.forEach((it, i) => {
			if (it.group && it.group !== group) {
				group = it.group;
				list.appendChild(h("div.palette-group", {}, group));
			}
			list.appendChild(h("div", { class: ["palette-item", i === focus && "focus"], role: "option", onmousemove: () => setFocus(i), onclick: () => run(it) }, icon(it.icon || "arrowRight"), h("span.grow.ellipsis", {}, it.label), it.sub ? h("span.kind", {}, it.sub) : null));
		});
		if (!items.length) list.appendChild(h("div.box-empty", {}, symbolMode ? "Type a symbol's name after #." : "Nothing matches."));
	};
	const setFocus = (i) => {
		if (i === focus) return;
		focus = i;
		const els = list.querySelectorAll(".palette-item");
		els.forEach((el, k) => el.classList.toggle("focus", k === i));
	};
	const fetchRemote = debounce(async () => {
		const q = input.value.trim();
		const my = ++seq;
		if (!q) {
			remote = [];
			draw();
			return;
		}
		try {
			if (q.startsWith("#")) {
				const name = q.slice(1).trim();
				if (!name) {
					remote = [];
				} else {
					const r = await api.get("/search", { q: name, limit: 12 });
					remote = (r.results || []).map((x) => ({ label: (x.recv ? x.recv + "." : "") + x.name, sub: `${x.owner}/${x.repo} · ${x.file}:${x.line}`, icon: x.kind === "type" ? "box" : "fn", href: `/${x.owner}/${x.repo}/blob/${x.commit}/${x.file}#L${x.line}`, group: "Symbols" }));
				}
			} else {
				const shown = repoShown;
				const [r, paths] = await Promise.all([api.get("/repos", { q, limit: 8 }), shown && q.length >= 2 ? filesOf(shown) : []]);
				const lq = q.toLowerCase();
				const files = paths
					.map((path) => ({ path, s: pathScore(path, lq) }))
					.filter((x) => x.s > 0)
					.sort((a, b) => b.s - a.s || a.path.length - b.path.length)
					.slice(0, 8)
					.map((x) => ({ label: x.path, icon: "file", href: `/${shown.owner}/${shown.name}/blob/${encodeURIComponent(shown.rev)}/${api.enc(x.path)}`, group: `Files in ${shown.owner}/${shown.name}` }));
				remote = [...files, ...(r.repos || []).map((x) => ({ label: `${x.owner}/${x.name}`, icon: x.visibility === "private" ? "lock" : "repo", href: `/${x.owner}/${x.name}`, group: "Repositories", sub: x.description ? x.description.slice(0, 40) : "" }))];
			}
		} catch {
			remote = [];
		}
		if (my === seq) draw();
	}, 140);
	const run = (it) => {
		close();
		if (it.run) it.run();
		else if (it.href) navigate(it.href);
	};
	input.addEventListener("input", () => {
		focus = 0;
		draw();
		fetchRemote();
	});
	input.addEventListener("keydown", (e) => {
		if (e.key === "ArrowDown") {
			e.preventDefault();
			setFocus(Math.min(items.length - 1, focus + 1));
			list.querySelectorAll(".palette-item")[focus]?.scrollIntoView({ block: "nearest" });
		} else if (e.key === "ArrowUp") {
			e.preventDefault();
			setFocus(Math.max(0, focus - 1));
			list.querySelectorAll(".palette-item")[focus]?.scrollIntoView({ block: "nearest" });
		} else if (e.key === "Enter") {
			e.preventDefault();
			if (items[focus]) run(items[focus]);
		} else if (e.key === "Escape") close();
	});
	const box = h("div.palette", { role: "dialog", "aria-label": "Command palette" }, h("div.palette-input", {}, icon("search", "lg"), input, h("kbd", {}, "Esc")), list, h("div.palette-foot", {}, h("span", {}, h("kbd", {}, "↑"), " ", h("kbd", {}, "↓"), " to move"), h("span", {}, h("kbd", {}, "Enter"), " to open"), h("span", {}, h("kbd", {}, "#"), " symbols")));
	const overlay = h("div.overlay", { onmousedown: (e) => e.target === overlay && close() }, box);
	function close() {
		overlay.remove();
		paletteOpen = null;
		if (prev && prev.focus) prev.focus();
	}
	paletteOpen = { close };
	document.body.appendChild(overlay);
	draw();
	if (initial) fetchRemote();
	setTimeout(() => input.focus(), 10);
}

// keys binds the global shortcuts: / and Ctrl+K open the palette (t too on a repository's pages, to find a file), g h
// home, g e explore.
export function keys() {
	let g = 0;
	document.addEventListener("keydown", (e) => {
		const t = e.target;
		const typing = t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.tagName === "SELECT" || t.isContentEditable);
		if ((e.key === "k" || e.key === "K") && (e.metaKey || e.ctrlKey)) {
			e.preventDefault();
			palette();
			return;
		}
		if (typing || e.metaKey || e.ctrlKey || e.altKey) return;
		if (e.key === "/" || (e.key === "t" && repoShown)) {
			e.preventDefault();
			palette();
		} else if (e.key === "g") g = Date.now();
		else if (Date.now() - g < 800) {
			if (e.key === "h") navigate("/");
			if (e.key === "e") navigate("/explore");
			if (e.key === "s") navigate("/search");
			g = 0;
		}
	});
}
