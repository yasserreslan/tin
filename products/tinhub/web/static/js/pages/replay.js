// Replay: production failures grouped by cause, each with its sealed capsules (a request and what it did), which a
// developer replays locally with `tit replay`. Reading capsules needs the replay permission.

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as fmt from "../lib/format.js";
import { setQuery } from "../lib/router.js";
import { frame } from "./repo.js";
import { checksPanel } from "../ui/checks.js";
import { badge, btn, busy, confirm, copyButton, empty, errorBox, kv, more, refused, segmented, signInFirst, skeleton, time, toast } from "../ui/kit.js";

const noAccess = (err) =>
	refused(err, "You can't read replays here", [err.message || "Reading capsules needs the replay permission.", " A repository admin grants it in Settings, Replay."], signInFirst("Sign in to read replays", "Replays hold production requests: reading them needs an account with the replay permission."));

function statusBadge(code) {
	if (!code) return null;
	return badge(String(code), code >= 500 ? "red" : code >= 400 ? "amber" : "outline");
}

function groupRow(base, g) {
	return h(
		"a.box-row.plain.group-row",
		{ href: `${base}/replay/${encodeURIComponent(g.id)}`, style: { "align-items": "flex-start", padding: "12px 16px" } },
		h("span", { class: g.state === "open" ? "red" : "green", style: { "margin-top": "2px" } }, icon(g.state === "open" ? "bug" : "checkCircle")),
		h(
			"div.grow",
			{ style: { "min-width": "0" } },
			h("div.panic", {}, g.panic || "(no panic message)"),
			h(
				"div.row.small.muted",
				{ style: { "margin-top": "6px", "flex-wrap": "wrap", gap: "8px" } },
				g.route ? h("span.mono", {}, g.route) : null,
				statusBadge(g.status),
				g.decl ? h("span", {}, "in ", h("span.mono", {}, g.decl)) : null,
				h("span", {}, "first ", time(g.first_at)),
				h("span", {}, "last ", time(g.last_at)),
				g.fixed_by ? h("span.green", {}, "fixed by ", h("span.mono", {}, fmt.shortChange(g.fixed_by))) : null,
				g.reopened_by ? h("span.amber", {}, "came back after ", h("span.mono", {}, fmt.shortChange(g.reopened_by))) : null,
			),
		),
		h("div.group-count", {}, h("b", {}, fmt.count(g.count)), h("span.tiny.muted", {}, g.held ? `${g.held} held` : "seen")),
	);
}

export async function render(ctx) {
	const { repo, body, base } = await frame(ctx, "replay");
	const R = api.R(repo.owner, repo.name);
	if (ctx.params.group) return group(ctx, repo, body, base, R, ctx.params.group);
	ctx.title("Replay", `${repo.owner}/${repo.name}`);
	const state = ctx.query.state ?? "open";
	const list = h("div.box", {}, skeleton(5));
	let moreBox = null;
	const load = (st) => {
		list.replaceChildren(skeleton(5));
		if (moreBox) moreBox.remove();
		let cursor = "";
		const page = async () => {
			const r = await api.get(R("/replay/groups"), { state: st, cursor, limit: 30 });
			if (!ctx.alive()) return false;
			if (!cursor) list.replaceChildren(h("div.box-head", {}, h("b", {}, st === "open" ? "Open failures" : st === "closed" ? "Fixed failures" : "All failures")));
			for (const g of r.groups || []) list.append(groupRow(base, g));
			cursor = r.next || "";
			if (list.childNodes.length === 1) list.append(empty("replay", st === "open" ? "No open failures" : "Nothing here", "When a deployed Tin server panics, it seals the request into a capsule and sends it here, grouped by cause."));
			return Boolean(cursor);
		};
		page().then(
			(again) => {
				if (again) {
					moreBox = more(page);
					list.after(moreBox);
				}
			},
			(err) => list.replaceChildren(err.status === 403 ? noAccess(err) : errorBox(err)),
		);
	};
	body.append(
		h(
			"div.page-head",
			{},
			h("div", {}, h("h2", {}, "Replay"), h("p.sub", {}, "Production failures, grouped by cause. Replay one locally with ", h("code", {}, "tit replay <capsule>"), ".")),
			segmented(
				[
					{ value: "open", label: "Open" },
					{ value: "closed", label: "Fixed" },
					{ value: "", label: "All" },
				],
				state,
				(v) => {
					setQuery({ state: v === "open" ? "" : v || "all" });
					load(v);
				},
			),
		),
		list,
	);
	load(state === "all" ? "" : state);
}

async function group(ctx, repo, body, base, R, id) {
	ctx.title("Failure " + id.slice(0, 10), `${repo.owner}/${repo.name}`);
	body.append(skeleton(8));
	let d;
	try {
		d = await api.get(R(`/replay/groups/${encodeURIComponent(id)}`));
	} catch (err) {
		if (err.status === 404) return ctx.notFound("There is no such failure group.");
		body.replaceChildren(err.status === 403 ? h("div.box", {}, noAccess(err)) : errorBox(err));
		return;
	}
	if (!ctx.alive()) return;
	const g = d.group;
	const admin = repo.role === "admin";
	const del = admin ? btn("Delete group", { danger: true, sm: true, icon: "trash" }) : null;
	if (del)
		del.onclick = () =>
			busy(del, async () => {
				if (!(await confirm({ title: "Delete this failure group?", message: "Its capsules are deleted too. A new failure with the same cause starts a new group.", confirmLabel: "Delete", danger: true }))) return;
				await api.del(R(`/replay/groups/${encodeURIComponent(id)}`));
				toast("Deleted");
				location.assign(`${base}/replay`);
			});
	const checks = checksPanel(ctx, repo, base, R, g, d.capsules || []);
	const rows = (d.capsules || []).map((c) => {
		const tr = h(
			"tr",
			{},
			h("td", {}, h("span.mono.small", {}, fmt.short(c.id, 12)), " ", copyButton(c.id, { title: "Copy the capsule id" })),
			h("td", {}, c.route ? h("span.mono.small", {}, c.route) : null, " ", statusBadge(c.status)),
			h("td", {}, c.commit ? h("a.hash", { href: `${base}/commit/${c.commit}` }, fmt.short(c.commit, 8)) : null),
			h("td.small", {}, c.signer || c.name || ""),
			h("td.num.small", {}, fmt.bytes(c.size_bytes)),
			h("td.small.muted", {}, time(c.created_at), c.expires_at ? h("div.tiny", {}, "expires ", time(c.expires_at)) : null),
			h(
				"td",
				{ style: { "white-space": "nowrap", "text-align": "right" } },
				btn("Check", { sm: true, ghost: true, icon: "play", title: "Replay this capsule against a branch or commit", onclick: () => checks.pick(c.id) }),
				h("a.btn.sm.ghost", { href: `/api/v1/repos/${repo.owner}/${repo.name}/replay/capsules/${encodeURIComponent(c.id)}/download`, "data-native": "", title: "Download the sealed capsule" }, icon("download", "sm")),
				admin
					? (() => {
							const b = btn("", { sm: true, ghost: true, icon: "trash", title: "Delete the capsule" });
							b.onclick = () =>
								busy(b, async () => {
									if (!(await confirm({ title: "Delete this capsule?", message: "It can't be replayed after this.", confirmLabel: "Delete", danger: true }))) return;
									await api.del(R(`/replay/capsules/${encodeURIComponent(c.id)}`));
									tr.remove();
									toast("Capsule deleted");
								});
							return b;
						})()
					: null,
			),
		);
		return tr;
	});
	body.replaceChildren(
		h("div.crumbs.mb-4", {}, h("a", { href: `${base}/replay` }, "Replay"), h("span.sep", {}, "/"), h("span.mono", {}, id.slice(0, 16))),
		h(
			"div.card.mb-4",
			{},
			h(
				"div.card-body",
				{},
				h("div.row", { style: { "align-items": "flex-start", gap: "12px" } }, h("span", { class: g.state === "open" ? "red" : "green" }, icon(g.state === "open" ? "bug" : "checkCircle")), h("div.grow", {}, h("div.panic", {}, g.panic || "(no panic message)"), h("div.row.small.muted", { style: { "margin-top": "8px", gap: "8px" } }, badge(g.state === "open" ? "Open" : "Fixed", g.state === "open" ? "red" : "green"), g.route ? h("span.mono", {}, g.route) : null, statusBadge(g.status))), del),
				h("div", { style: { "margin-top": "16px" } }, kv([
					["Seen", fmt.plural(g.count, "time")],
					["Capsules held", String(g.held)],
					["First", time(g.first_at)],
					["Last", time(g.last_at)],
					g.decl ? ["Where", h("span.mono", {}, g.decl)] : null,
					g.commit ? ["Commit", h("a.hash", { href: `${base}/commit/${g.commit}` }, fmt.short(g.commit, 10))] : null,
					g.fixed_by ? ["Fixed by", h("a.mono", { href: `${base}/change/${g.fixed_by}` }, fmt.shortChange(g.fixed_by))] : null,
					g.reopened_by ? ["Came back after", h("a.mono", { href: `${base}/change/${g.reopened_by}` }, fmt.shortChange(g.reopened_by))] : null,
				])),
			),
		),
		checks,
		h(
			"div.box",
			{},
			h("div.box-head", {}, h("b", {}, fmt.plural(rows.length, "capsule")), h("span.spacer"), h("span.small.muted", {}, "Replay one with ", h("code", {}, "tit replay <id>"))),
			rows.length ? h("div", { style: { overflow: "auto" } }, h("table.table", {}, h("thead", {}, h("tr", {}, ["Capsule", "Request", "Commit", "Signer", "Size", "Sealed", ""].map((t, i) => h("th", { class: i === 4 ? "num" : "" }, t)))), h("tbody", {}, rows))) : empty("capsule", "No capsules held", "They expired, or were deleted. The group keeps counting new failures."),
		),
	);
}
