// Activity: the repository's pushes and review events, newest first, live.

import { h } from "../lib/dom.js";
import * as api from "../lib/api.js";
import * as feed from "../ui/feed.js";
import { frame, onLive } from "./repo.js";
import { empty, errorBox, more, skeleton } from "../ui/kit.js";

export async function render(ctx) {
	const { repo, body } = await frame(ctx, "activity");
	ctx.title("Activity", `${repo.owner}/${repo.name}`);
	const items = h("div");
	const card = h("div.card", {}, h("div.card-body", { style: { padding: "8px 20px" } }, items));
	body.append(h("div.page-head", {}, h("div", {}, h("h2", {}, "Activity"), h("p.sub", {}, "Pushes, reviews, votes, checks and landings."))), card);
	items.append(skeleton(8));
	let cursor = "";
	let moreBox = null;
	const load = async (reset = false) => {
		if (reset) cursor = "";
		const r = await api.get(api.R(repo.owner, repo.name)("/events"), { cursor, limit: 40 });
		if (!ctx.alive()) return false;
		if (!cursor) items.replaceChildren();
		for (const e of r.events || []) items.append(feed.item(e, { showRepo: false }));
		cursor = r.next || "";
		if (!items.childNodes.length) items.append(empty("activity", "No activity yet", "Push to the repository and its history shows up here."));
		return Boolean(cursor);
	};
	try {
		if (await load()) {
			moreBox = more(load);
			card.after(moreBox);
		}
	} catch (err) {
		items.replaceChildren(errorBox(err));
	}
	onLive(ctx, repo, () => {
		if (moreBox) moreBox.remove();
		load(true).then((again) => {
			if (again) {
				moreBox = more(load);
				card.after(moreBox);
			}
		}, () => {});
	});
}
