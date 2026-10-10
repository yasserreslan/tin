// Reviews: the repository's reviews by state, newest activity first.

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as fmt from "../lib/format.js";
import { setQuery } from "../lib/router.js";
import { frame, onLive } from "./repo.js";
import { title } from "../ui/commits.js";
import { avatar, empty, skeleton, stateBadge, time, errorBox, segmented, badge } from "../ui/kit.js";

// "Open" is every review still to land: waiting, approved or with changes requested, what the tab's count counts
const STATES = [
	{ value: "active", label: "Open" },
	{ value: "approved", label: "Approved" },
	{ value: "changes_requested", label: "Changes requested" },
	{ value: "landed", label: "Landed" },
	{ value: "abandoned", label: "Abandoned" },
	{ value: "", label: "All" },
];

const ICON = { open: ["review", "green"], approved: ["checkCircle", "blue"], changes_requested: ["alert", "amber"], landed: ["landed", "purple"], abandoned: ["xCircle", "muted"] };

export function reviewRow(repo, base, r) {
	const [ic, col] = ICON[r.state] || ["review", ""];
	return h(
		"a.box-row.plain",
		{ href: `${base}/change/${r.change}`, style: { "align-items": "flex-start", padding: "12px 16px" } },
		h("span", { class: col, style: { "margin-top": "2px" } }, icon(ic)),
		h(
			"div.grow",
			{ style: { "min-width": "0" } },
			h("div.title.ellipsis", {}, title(repo, r.commit, fmt.shortChange(r.change))),
			h("div.row.small.muted", { style: { "margin-top": "4px", "flex-wrap": "wrap", gap: "6px" } }, h("span.mono", {}, fmt.shortChange(r.change)), h("span", {}, "·"), r.opened_by ? [h("span", {}, "opened by ", h("b", {}, r.opened_by)), h("span", {}, "·")] : null, h("span", {}, "into ", h("span.mono", {}, r.target.replace("refs/heads/", ""))), h("span", {}, "·"), h("span", {}, "updated ", time(r.updated_at))),
		),
		h("span.badge.outline", { title: "Newest version" }, "v" + r.version),
		r.opened_by ? avatar(r.opened_by, "sm") : null,
	);
}

export async function render(ctx) {
	const { repo, body, base } = await frame(ctx, "reviews");
	ctx.title("Reviews", `${repo.owner}/${repo.name}`);
	let state = ctx.query.state ?? "active";
	const list = h("div.box", {}, skeleton(5));
	const load = async (st) => {
		list.replaceChildren(skeleton(5));
		try {
			const r = await api.get(api.R(repo.owner, repo.name)("/reviews"), { state: st });
			if (!ctx.alive()) return;
			const rs = r.reviews || [];
			list.replaceChildren(
				h("div.box-head", {}, h("b", {}, fmt.plural(rs.length, "review")), h("span.spacer"), h("span.small.muted", {}, "Push a branch with ", h("code", {}, "tit push"), " to open reviews for its changes.")),
				...rs.map((x) => reviewRow(repo, base, x)),
			);
			if (!rs.length) list.append(empty("review", st ? `No ${STATES.find((s) => s.value === st)?.label.toLowerCase()} reviews` : "No reviews yet", "Each change you push on a branch above the trunk gets a review here."));
		} catch (err) {
			list.replaceChildren(errorBox(err));
		}
	};
	body.append(
		h(
			"div.page-head",
			{ style: { "align-items": "center" } },
			h("h2", {}, "Reviews"),
			h(
				"div.actions",
				{},
				segmented(STATES, state === "all" ? "" : state, (v) => {
					state = v || "all";
					setQuery({ state: v === "active" ? "" : state });
					load(v);
				}),
			),
		),
		list,
	);
	await load(state === "all" ? "" : state);
	onLive(ctx, repo, (e) => {
		if (e.kind.startsWith("review.")) load(state === "all" ? "" : state);
	});
}
