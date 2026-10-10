// Symbol search across repositories (index/search.tin): "fn Name", "type Name", "Recv.Name" or a name prefix.

import { h, debounce } from "../lib/dom.js";
import * as api from "../lib/api.js";
import { setQuery } from "../lib/router.js";
import { icon } from "../lib/icons.js";
import { empty, skeleton, more, errorBox, badge } from "../ui/kit.js";

function hit(x) {
	const href = `/${x.owner}/${x.repo}/blob/${x.commit}/${x.file}#L${x.line}`;
	return h(
		"a.hit.plain",
		{ href, style: { display: "block" } },
		h("div.row", {}, badge(x.kind, x.kind === "type" ? "purple" : x.kind === "fn" || x.kind === "method" ? "blue" : ""), h("span.sig", {}, x.recv ? [h("span.muted", {}, x.recv + "."), h("b", {}, x.name)] : h("b", {}, x.name))),
		h("div.where", {}, x.owner, " / ", h("b", {}, x.repo), " · ", h("span.mono", {}, `${x.package ? x.package + " · " : ""}${x.file}:${x.line}`)),
	);
}

export async function render(ctx) {
	ctx.title("Search");
	const q = h("input.input", { placeholder: "Search symbols: fn Push, type Repo, Repo.Push, or a name", value: ctx.query.q || "", autofocus: true, style: { height: "42px", "font-size": "15px", "padding-left": "38px" } });
	const list = h("div.box");
	const info = h("div.small.muted.mb-4");
	ctx.main.replaceChildren(
		h(
			"div.container.narrow.page",
			{},
			h("div.page-head", {}, h("div", {}, h("h1", {}, "Search"), h("p.sub", {}, "Find functions, methods, types and constants in every repository you can read."))),
			h("div.input-icon.mb-4", {}, icon("search"), q),
			info,
			list,
		),
	);
	let cursor = "";
	let token = 0;
	let moreEl = null;
	const load = async () => {
		const my = token;
		const r = await api.get("/search", { q: q.value.trim(), cursor, limit: 30 });
		if (my !== token) return false;
		if (!cursor) list.replaceChildren();
		for (const x of r.results || []) list.append(hit(x));
		if (!list.childNodes.length) list.append(empty("search", "No symbols found", "Repositories are indexed after each push; try a shorter name."));
		cursor = r.next || "";
		info.textContent = "";
		return Boolean(cursor);
	};
	const run = async () => {
		token++;
		cursor = "";
		if (moreEl) moreEl.remove();
		if (!q.value.trim()) {
			list.replaceChildren(empty("search", "Search the code", "Type a name. Prefix with fn or type to narrow it, or write Recv.Name for a method."));
			return;
		}
		list.replaceChildren(skeleton(5));
		try {
			if (await load()) list.after((moreEl = more(load)));
		} catch (err) {
			list.replaceChildren(errorBox(err, run));
		}
	};
	q.addEventListener(
		"input",
		debounce(() => {
			setQuery({ q: q.value.trim() });
			run();
		}, 220),
	);
	setTimeout(() => q.focus(), 10);
	await run();
}
