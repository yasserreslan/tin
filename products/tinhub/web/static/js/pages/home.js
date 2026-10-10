// The home page: the landing for visitors, the dashboard (your repositories, orgs and activity) for the signed in.

import { h, replace } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as store from "../lib/store.js";
import { btn, empty, skeleton, avatar, more, errorBox, copyButton } from "../ui/kit.js";
import * as feed from "../ui/feed.js";
import * as repos from "../ui/repos.js";

export async function render(ctx) {
	if (store.session.user) return dashboard(ctx, store.session.user);
	return landing(ctx);
}

const FEATURES = [
	["review", "Reviews of changes, not branches", "Every change keeps its id across rebases. Reviews follow it from version to version, and comments stay on the declaration they were about."],
	["code", "Diffs that understand Tin", "See which functions and types a change added, removed, moved or renamed, with line diffs inside each declaration."],
	["stack", "Stacks that land together", "Push a stack of dependent changes, review each one, and land the whole stack bottom-up in one go."],
	["key", "Keys, not passwords", "Your tit signing key is your identity. Sign in to the web by approving a code from your terminal."],
	["gauge", "Benchmarks on every change", "Linux benchmark results per commit, with a verdict on each review when a change makes something slower."],
	["replay", "Replay production failures", "Panics captured in production become capsules you can replay locally, grouped by cause, linked to the fix."],
];

function landing(ctx) {
	ctx.title("");
	const site = store.site.url;
	ctx.main.replaceChildren(
		h(
			"section.hero",
			{},
			h(
				"div.container",
				{},
				h("div.badge.accent.lg", { style: { "margin-bottom": "20px" } }, icon("sparkle"), "Hosting for tit repositories"),
				h("h1", {}, "Code review for ", h("span.grad", {}, "changes that evolve"), "."),
				h("p.lead", {}, "tinhub hosts tit repositories: browse code, review changes version by version with diffs that know your declarations, land stacks, and keep an eye on performance. One binary, one database."),
				h("div.cta", {}, btn("Sign in", { primary: true, lg: true, icon: "login", href: "/login" }), btn("Explore repositories", { lg: true, icon: "globe", href: "/explore" })),
				h(
					"div.terminal",
					{},
					h("div.bar", {}, h("i"), h("i"), h("i")),
					h(
						"pre",
						{},
						h("span.c", {}, "# get the code"),
						"\n",
						h("span.p", {}, "$ "),
						`tit clone ${site}/owner/repo`,
						"\n",
						h("span.c", {}, "# change it, then send it for review"),
						"\n",
						h("span.p", {}, "$ "),
						"tit commit -m \"anvil: stream large bodies\"",
						"\n",
						h("span.p", {}, "$ "),
						"tit push",
						"\n",
						h("span.c", {}, "# sign this browser in with your key"),
						"\n",
						h("span.p", {}, "$ "),
						`tit login ${site} --code ABCD-EFGH`,
					),
				),
			),
		),
		h("section.container.page", {}, h("div.features", {}, FEATURES.map(([ic, t, d]) => h("div.feature", {}, h("div.ficon", {}, icon(ic, "lg")), h("h3", {}, t), h("p", {}, d))))),
	);
}

async function dashboard(ctx, u) {
	ctx.title("Dashboard");
	const left = h("aside.col", { style: { gap: "20px" } });
	const center = h("div");
	ctx.main.replaceChildren(h("div.container.page", {}, h("div.layout-sidebar.wide.feed-first", {}, left, center)));

	// your repositories
	const repoBox = h("div.box", {}, h("div.box-head", {}, h("b.grow", {}, "Your repositories"), btn("New", { sm: true, primary: true, icon: "plus", href: "/new" })), skeleton(5));
	const filter = h("input.input", { placeholder: "Find a repository…", style: { height: "30px", "font-size": "12.5px" } });
	left.append(repoBox);
	const orgBox = (u.orgs || []).length
		? h("div.box", {}, h("div.box-head", {}, h("b.grow", {}, "Organizations"), btn("", { sm: true, ghost: true, icon: "plus", href: "/new/org", title: "New organization" })), (u.orgs || []).map((o) => h("a.box-row.plain", { href: "/" + o.name }, avatar(o.name, "sm", { square: true }), h("span.grow.ellipsis", {}, o.display || o.name), h("span.badge.outline", {}, o.role))))
		: null;
	if (orgBox) left.append(orgBox);
	const recent = store.recent();
	if (recent.length) left.append(h("div.box", {}, h("div.box-head", {}, h("b", {}, "Recently viewed")), recent.slice(0, 6).map((r) => h("a.box-row.plain", { href: "/" + r }, icon("history", "sm"), h("span.ellipsis", {}, r)))));

	api.get("/user/repos").then(
		(r) => {
			const list = r.repos || [];
			const rows = h("div");
			const draw = () => {
				const q = filter.value.toLowerCase();
				rows.replaceChildren(
					...list
						.filter((x) => !q || `${x.owner}/${x.name}`.toLowerCase().includes(q))
						.slice(0, 30)
						.map((x) => h("a.box-row.plain", { href: `/${x.owner}/${x.name}`, style: { "min-height": "38px", padding: "7px 16px" } }, avatar(x.owner, "sm", { square: true }), h("span.grow.ellipsis.small", {}, x.owner, h("span.faint", {}, " / "), h("b", {}, x.name)), x.visibility === "private" ? icon("lock", "sm") : null)),
				);
			};
			filter.oninput = draw;
			draw();
			replace(repoBox, repoBox.firstChild, list.length ? [h("div", { style: { padding: "8px 12px 4px" } }, filter), rows] : h("div.box-empty.small", {}, "You have no repositories yet. ", h("a", { href: "/new" }, "Create one"), "."));
		},
		(err) => replace(repoBox, repoBox.firstChild, h("div.box-empty.small", {}, err.message)),
	);

	// activity
	const items = h("div");
	center.append(
		h("div.page-head", {}, h("div", {}, h("h1", {}, "Welcome back, ", u.display || u.name), h("p.sub", {}, "What happened in the repositories you work on and follow."))),
		h("div.card", {}, h("div.card-body", { style: { padding: "8px 20px" } }, items)),
	);
	items.append(skeleton(6));
	let cursor = "";
	const load = async () => {
		const r = await api.get("/feed", { cursor, limit: 30 });
		if (!cursor) items.replaceChildren();
		for (const e of r.events || []) items.append(feed.item(e));
		cursor = r.next || "";
		if (!items.childNodes.length) items.append(empty("activity", "Nothing yet", "Pushes, reviews and landings in your repositories and the ones you follow show up here.", btn("Explore repositories", { href: "/explore", icon: "globe" })));
		return Boolean(cursor);
	};
	try {
		if (await load()) center.querySelector(".card").after(more(load));
	} catch (err) {
		items.replaceChildren(errorBox(err));
	}
}
