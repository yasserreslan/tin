// Explore: every repository the visitor can read, newest push first, with a filter.

import { h, debounce } from "../lib/dom.js";
import * as api from "../lib/api.js";
import { setQuery } from "../lib/router.js";
import { icon } from "../lib/icons.js";
import { empty, skeleton, more, errorBox, segmented } from "../ui/kit.js";
import * as repos from "../ui/repos.js";
import * as store from "../lib/store.js";

export async function render(ctx) {
	ctx.title("Explore");
	const q = h("input.input", { placeholder: "Filter repositories by name or description", value: ctx.query.q || "", "aria-label": "Filter" });
	let view = store.pref("explore.view", "grid");
	const list = h("div");
	const wrap = h("div", {}, list);
	ctx.main.replaceChildren(
		h(
			"div.container.page",
			{},
			h("div.page-head", {}, h("div", {}, h("h1", {}, "Explore"), h("p.sub", {}, "Repositories hosted here that you can read."))),
			h(
				"div.row.mb-4",
				{},
				h("div.input-icon.grow", {}, icon("search", "sm"), q),
				segmented(
					[
						{ value: "grid", icon: "grid", title: "Cards" },
						{ value: "list", icon: "list", title: "List" },
					],
					view,
					(v) => {
						view = v;
						store.setPref("explore.view", v);
						reset();
					},
				),
			),
			wrap,
		),
	);
	let cursor = "";
	let token = 0;
	let moreEl = null;
	const load = async () => {
		const my = token;
		const r = await api.get("/repos", { q: q.value.trim(), cursor, limit: 48 });
		if (my !== token) return false;
		const rs = r.repos || [];
		if (!cursor) {
			list.replaceChildren();
			list.className = view === "grid" ? "card-grid" : "box";
		}
		for (const x of rs) list.append(view === "grid" ? repos.card(x) : repos.row(x));
		if (!list.childNodes.length) {
			list.className = "";
			list.append(empty("repo", q.value ? "No repositories match" : "No repositories yet", q.value ? "Try a different filter." : "Create the first one.", null));
		}
		cursor = r.next || "";
		return Boolean(cursor);
	};
	const reset = async () => {
		token++;
		cursor = "";
		if (moreEl) moreEl.remove();
		list.className = "";
		list.replaceChildren(skeleton(6));
		try {
			if (await load()) wrap.append((moreEl = more(load)));
		} catch (err) {
			list.replaceChildren(errorBox(err, reset));
		}
	};
	q.addEventListener(
		"input",
		debounce(() => {
			setQuery({ q: q.value.trim() });
			reset();
		}, 200),
	);
	await reset();
}
