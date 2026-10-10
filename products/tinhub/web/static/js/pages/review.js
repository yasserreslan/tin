// One change and its review: the conversation (description, merge box, timeline, comments), the semantic diff of
// any version against its base or the version before, checks, benchmarks and behaviour; votes, landing, the stack.

import { h, debounce } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as fmt from "../lib/format.js";
import * as df from "../lib/diff.js";
import * as live from "../lib/live.js";
import * as store from "../lib/store.js";
import { navigate, setQuery } from "../lib/router.js";
import { frame, forget } from "./repo.js";
import { commitOf } from "../ui/commits.js";
import { chains } from "./stacks.js";
import { markdown, hunkTable, fileBlock } from "../ui/code.js";
import { composer } from "../ui/composer.js";
import { threads, thread } from "../ui/comments.js";
import * as filediff from "../ui/filediff.js";
import { avatar, badge, btn, busy, callout, confirm, copyButton, empty, errorBox, segmented, skeleton, stateBadge, tabs, time, toast, kv } from "../ui/kit.js";
import { line as lineChart } from "../ui/chart.js";

const DECL_COLOR = { added: "green", removed: "red", changed: "amber", moved: "blue", renamed: "purple" };

export async function render(ctx) {
	const { repo, body, base } = await frame(ctx, "reviews");
	const ch = ctx.params.id;
	const tab = ctx.params.tab || "conversation";
	const R = api.R(repo.owner, repo.name);
	const C = (rest = "") => R(`/changes/${encodeURIComponent(ch)}${rest}`);
	const me = store.session.user;
	const canWrite = repo.role === "write" || repo.role === "admin";
	body.append(skeleton(8));

	const state = { review: null, versions: [], comments: [], votes: { votes: [], counted: {} }, commit: null };
	const loadAll = async () => {
		const [review, versions, comments, votes] = await Promise.all([
			api.get(C("/review")).catch((e) => (e.status === 404 ? null : Promise.reject(e))),
			api.get(C()),
			api.get(C("/comments")).catch(() => ({ comments: [] })),
			api.get(C("/approvals")).catch(() => ({ votes: [], counted: {} })),
		]);
		state.review = review;
		state.versions = versions.versions || [];
		state.comments = comments.comments || [];
		state.votes = votes;
		const newest = state.versions[state.versions.length - 1];
		state.commit = newest ? await commitOf(repo, newest.commit).catch(() => null) : null;
	};
	await loadAll();
	if (!ctx.alive()) return;
	const newest = state.versions[state.versions.length - 1];
	const titleText = state.commit ? fmt.title(state.commit.message) : fmt.shortChange(ch);
	ctx.title(titleText, `${repo.owner}/${repo.name}`);

	const head = h("div");
	const sidebar = h("aside");
	const content = h("div");
	const tabBar = h("div");
	body.replaceChildren(head, tabBar, h("div.layout-aside", { style: { "margin-top": "20px" } }, content, sidebar));

	const opener = () => (state.review && state.review.opened_by_name) || (state.versions[0] && state.versions[0].pushed_by) || "";

	const drawHead = () => {
		const rv = state.review;
		const target = rv ? rv.target.replace("refs/heads/", "") : "";
		head.replaceChildren(
			h(
				"div.review-head",
				{},
				h(
					"div.grow",
					{ style: { "min-width": "0" } },
					h("h1", {}, titleText, " ", h("span.faint", { style: { "font-weight": "400" } }, fmt.shortChange(ch))),
					h(
						"div.meta",
						{},
						rv ? stateBadge(rv.state, true) : badge("No review", "outline lg"),
						opener() ? h("b", {}, opener()) : null,
						rv ? [rv.state === "landed" ? "landed this into " : "wants to land this into ", h("a.chip", { href: `${base}/tree/${encodeURIComponent(target)}` }, icon("branch", "sm"), target)] : "pushed this change",
						h("span.faint", {}, "·"),
						fmt.plural(state.versions.length, "version"),
						h("span.faint", {}, "·"),
						rv ? time(rv.updated_at, { prefix: "updated " }) : newest ? time(newest.time) : null,
					),
				),
				h("div.row", {}, h("span.chip", { title: ch }, icon("change", "sm"), ch), copyButton(ch, { title: "Copy the change id" }), me ? followButton() : null),
			),
		);
	};

	let following = false;
	const followButton = () => {
		const b = btn("Follow", { icon: "bell", sm: true });
		b.onclick = () =>
			busy(b, async () => {
				if (following) await api.del(C("/subscription"));
				else await api.put(C("/subscription"));
				following = !following;
				b.replaceChildren(icon(following ? "bellOff" : "bell", "sm"), following ? "Unfollow" : "Follow");
				toast(following ? "You'll be notified about this change." : "Stopped following this change.");
			});
		api.get("/subscriptions").then((s) => {
			following = (s.subscriptions || []).some((x) => x.repo === `${repo.owner}/${repo.name}` && x.change === ch);
			if (following) b.replaceChildren(icon("bellOff", "sm"), "Unfollow");
		}, () => {});
		return b;
	};

	const drawTabs = () => {
		const n = state.comments.length;
		tabBar.replaceChildren(
			tabs([
				{ label: "Conversation", icon: "comments", href: `${base}/change/${ch}`, active: tab === "conversation", count: n },
				{ label: "Files", icon: "fileCode", href: `${base}/change/${ch}/files`, active: tab === "files" },
				{ label: "Checks", icon: "checkCircle", href: `${base}/change/${ch}/checks`, active: tab === "checks", count: state.review ? state.review.checks.filter((c) => c.version === newest.version).length : 0 },
				{ label: "Bench", icon: "gauge", href: `${base}/change/${ch}/bench`, active: tab === "bench" },
				{ label: "Behaviour", icon: "runner", href: `${base}/change/${ch}/behaviour`, active: tab === "behaviour" },
			]),
		);
	};

	// ---- actions ----

	const reload = async () => {
		await loadAll();
		if (!ctx.alive()) return;
		forget(repo.owner, repo.name);
		drawHead();
		drawTabs();
		drawSidebar();
		if (tab === "conversation") drawConversation();
		if (tab === "files" && filesRedraw) filesRedraw();
	};
	const vote = (v, b) =>
		busy(b, async () => {
			await api.post(C("/approvals"), { vote: v });
			toast(v === "approve" ? "Approved" : "Changes requested");
			await reload();
		});
	const setState = (s, b) =>
		busy(b, async () => {
			if (s === "abandoned" && !(await confirm({ title: "Abandon this change?", message: "Its review closes. Pushing a new version, or reopening it, opens it again.", confirmLabel: "Abandon", danger: true }))) return;
			await api.patch(C("/review"), { state: s });
			toast(s === "abandoned" ? "Abandoned" : "Reopened");
			await reload();
		});
	const land = (stack, b) =>
		busy(b, async () => {
			const ok = await confirm({ title: stack ? "Land the stack?" : "Land this change?", message: stack ? "Every change below this one lands first, bottom up, each rebased on the target as needed. One that cannot land stops the rest." : `The change is rebased on ${state.review.target.replace("refs/heads/", "")} if needed and the branch moves to it.`, confirmLabel: stack ? "Land stack" : "Land", typed: "" });
			if (!ok) return;
			let r;
			try {
				r = await api.post(C("/land"), { stack });
			} catch (err) {
				toast(err.message, "error");
				await reload();
				return;
			}
			if (r.code) toast(r.message || r.code, "error");
			else toast(`Landed ${fmt.plural((r.landed || []).length, "change")}`);
			await reload();
		});
	const addComment = async (bodyText, anchor = {}) => {
		await api.post(C("/comments"), { body: bodyText, file: anchor.file || "", decl: anchor.decl || "", line_offset: anchor.line_offset || 0, version: anchor.version || newest.version, parent: anchor.parent || 0 });
		await reload();
	};
	const commentAct = {
		canWrite: Boolean(me),
		reply: (root, text) => addComment(text, { file: root.file, decl: root.decl, line_offset: root.line_offset, version: root.version, parent: root.id }),
		resolve: async (root, v) => {
			await api.patch(C(`/comments/${root.id}`), { resolved: v });
			await reload();
		},
		link: (c) => `${base}/change/${ch}/files#${encodeURIComponent(c.file)}`,
	};

	// ---- sidebar ----

	const section = (title, ...kids) => h("div.sidebar-section", {}, h("h4", {}, title), kids);
	const drawSidebar = () => {
		const rv = state.review;
		const votes = state.votes.votes || [];
		const counted = state.votes.counted || {};
		const voteRows = votes.length
			? votes.map((v) => h("div.row", {}, avatar(v.name, "sm"), h("b.grow.ellipsis", {}, v.name), h("span", { class: v.vote === "approve" ? "green" : "amber", title: v.vote === "approve" ? "Approved" : "Requested changes" }, icon(v.vote === "approve" ? "checkCircle" : "alert", "sm")), h("span.tiny.faint", { title: v.version < newest.version ? "On an older version" : "" }, "v" + v.version)))
			: h("p.small.muted", {}, "No votes yet.");
		// the caller's own vote on the newest version
		const mineV = me ? votes.find((v) => v.name === me.name && v.version === newest.version) : null;
		const mine = mineV ? mineV.vote : "";
		const voteBtns =
			me && rv && rv.state !== "landed" && rv.state !== "abandoned"
				? h(
						"div.row",
						{ style: { "margin-top": "10px" } },
						(() => {
							const b = btn(mine === "approve" ? "Approved" : "Approve", { sm: true, success: mine !== "approve", icon: "thumbsUp", class: mine === "approve" ? "selected" : "" });
							b.onclick = () => vote("approve", b);
							return b;
						})(),
						(() => {
							const b = btn(mine === "changes" ? "Changes requested" : "Request changes", { sm: true, icon: "thumbsDown", class: mine === "changes" ? "selected" : "" });
							b.onclick = () => vote("changes", b);
							return b;
						})(),
					)
				: null;
		const versionRows = [...state.versions].reverse().map((v) => h("div.row.small", {}, h("span.badge.outline", {}, "v" + v.version), h("a.hash.grow", { href: `${base}/commit/${v.commit}` }, fmt.short(v.commit, 8)), h("span.muted.ellipsis", {}, v.pushed_by), time(v.time)));
		const stackBox = h("div", {}, h("p.small.muted", {}, "…"));
		const overlapBox = h("div", {}, h("p.small.muted", {}, "…"));
		const actions = [];
		if (rv && me) {
			if (rv.state === "abandoned") {
				const b = btn("Reopen", { sm: true, icon: "refresh" });
				b.onclick = () => setState("open", b);
				actions.push(b);
			} else if (rv.state !== "landed") {
				const b = btn("Abandon", { sm: true, danger: true, icon: "xCircle" });
				b.onclick = () => setState("abandoned", b);
				actions.push(b);
			}
		}
		sidebar.replaceChildren(
			rv ? section(h("span.grow", {}, "Approvals"), h("div.small.muted", { style: { "margin-bottom": "8px" } }, `${counted.approvals || 0} of ${counted.needed || 0} needed`, counted.blocking ? h("span.amber", {}, ` · ${counted.blocking} blocking`) : null), voteRows, voteBtns) : null,
			section("Stack", stackBox),
			section("Overlaps", overlapBox),
			section("Versions", versionRows),
			actions.length ? section("Actions", h("div.row", {}, actions)) : null,
		);
		// the stack this change is in (the pusher's stacks)
		const who = (newest && newest.pushed_by) || opener();
		if (who)
			api.get(R(`/stacks/${encodeURIComponent(who)}`), { limit: 200 }).then(
				(r) => {
					const chain = chains(r.changes || []).find((c) => c.some((x) => x.change === ch));
					if (!chain || chain.length < 2) {
						stackBox.replaceChildren(h("p.small.muted", {}, "Not part of a stack."));
						return;
					}
					stackBox.replaceChildren(
						h(
							"div.col",
							{ style: { gap: "2px" } },
							[...chain].reverse().map((x, i) =>
								h(
									"a.row.plain.small",
									{ href: `${base}/change/${x.change}`, style: { padding: "4px 6px", "border-radius": "6px", background: x.change === ch ? "var(--accent-soft)" : null } },
									h("span.mono.faint", {}, String(chain.length - i)),
									h("span.grow.ellipsis", { style: { "font-weight": x.change === ch ? "600" : "400" } }, commitTitle(x.newest.commit, fmt.shortChange(x.change))),
								),
							),
						),
					);
				},
				() => stackBox.replaceChildren(h("p.small.muted", {}, "Unavailable.")),
			);
		else stackBox.replaceChildren(h("p.small.muted", {}, "Not part of a stack."));
		api.get(C("/overlaps")).then(
			(o) => {
				const list = o.overlaps || [];
				overlapBox.replaceChildren(
					list.length
						? h("div.col", { style: { gap: "6px" } }, list.map((x) => h("div.small", {}, h("a.mono", { href: `${base}/change/${x.change}` }, fmt.shortChange(x.change)), h("div.tiny.muted", {}, (x.decls || []).slice(0, 4).join(", "), (x.decls || []).length > 4 ? "…" : ""))))
						: h("p.small.muted", {}, "No other open change touches these declarations."),
				);
			},
			() => overlapBox.replaceChildren(h("p.small.muted", {}, "Unavailable.")),
		);
	};

	const commitTitle = (id, fallback) => {
		const el = h("span", {}, fallback);
		commitOf(repo, id).then((c) => c && (el.textContent = fmt.title(c.message)), () => {});
		return el;
	};

	// ---- conversation ----

	const mergeBox = () => {
		const rv = state.review;
		if (!rv) return callout("info", "This change has no review: it was pushed to the target directly, or its branch is the trunk.");
		const counted = rv.votes || {};
		const versionChecks = rv.checks.filter((c) => c.version === rv.version);
		const required = rv.required || [];
		const rows = [];
		if (rv.state === "landed") {
			rows.push(h("div.merge-row", {}, h("div.merge-icon", { style: { background: "var(--purple-soft)", color: "var(--purple)" } }, icon("landed", "sm")), h("div", {}, h("b", {}, "Landed"), h("div.small.muted", {}, rv.landed_by_name ? [h("b", {}, rv.landed_by_name), " landed it as "] : "Landed as ", h("a.hash", { href: `${base}/commit/${rv.landed_commit}` }, fmt.short(rv.landed_commit, 10)), " ", time(rv.landed_at)))));
			return h("div.merge-box", {}, rows);
		}
		if (rv.state === "abandoned") return h("div.merge-box", {}, h("div.merge-row", {}, h("div.merge-icon.info", {}, icon("xCircle", "sm")), h("div", {}, h("b", {}, "Abandoned"), h("div.small.muted", {}, "Reopen it, or push a new version, to review it again."))));
		const approved = (counted.approvals || 0) >= (counted.needed || 0) && !(counted.blocking > 0);
		rows.push(h("div.merge-row", {}, h("div", { class: ["merge-icon", approved ? "ok" : counted.blocking ? "bad" : "wait"] }, icon(approved ? "check" : counted.blocking ? "x" : "clock", "sm")), h("div", {}, h("b", {}, approved ? "Approved" : counted.blocking ? "Changes requested" : "Waiting for approvals"), h("div.small.muted", {}, `${counted.approvals || 0} of ${counted.needed || 0} approvals on the newest version`, counted.blocking ? `, ${counted.blocking} asking for changes` : ""))));
		const failing = versionChecks.filter((c) => c.state === "failure" || c.state === "error");
		const pending = versionChecks.filter((c) => c.state === "pending");
		const missing = required.filter((n) => !versionChecks.some((c) => c.name === n));
		const checksOk = !failing.length && !pending.length && !missing.length;
		rows.push(
			h(
				"div.merge-row",
				{},
				h("div", { class: ["merge-icon", checksOk ? "ok" : failing.length ? "bad" : "wait"] }, icon(checksOk ? "check" : failing.length ? "x" : "clock", "sm")),
				h(
					"div.grow",
					{},
					h("b", {}, checksOk ? (versionChecks.length ? "Checks passed" : "No checks required") : failing.length ? `${fmt.plural(failing.length, "check")} failed` : missing.length ? `Waiting for ${missing.join(", ")}` : `${fmt.plural(pending.length, "check")} running`),
					versionChecks.length ? h("div", { style: { "margin-top": "6px" } }, versionChecks.map(checkRow)) : null,
				),
			),
		);
		const ready = !rv.blocked;
		rows.push(h("div.merge-row", {}, h("div", { class: ["merge-icon", ready ? "ok" : "info"] }, icon(ready ? "landed" : "info", "sm")), h("div", {}, h("b", {}, ready ? "Ready to land" : "Not ready to land"), h("div.small.muted", {}, ready ? `Landing rebases the change onto ${rv.target.replace("refs/heads/", "")} if it has moved.` : rv.blocked))));
		const foot = h("div.merge-foot");
		if (canWrite) {
			const l = btn("Land", { success: true, icon: "landed", disabled: !ready });
			l.onclick = () => land(false, l);
			const ls = btn("Land stack", { icon: "stack", title: "Land every change below this one, then this one" });
			ls.onclick = () => land(true, ls);
			foot.append(l, ls, h("span.small.muted", {}, ready ? "" : "The stack can still land when every change in it is ready."));
		} else foot.append(h("span.small.muted", {}, me ? "Only people with write access can land changes." : "Sign in to vote or comment."));
		return h("div.merge-box", {}, rows, foot);
	};

	const drawConversation = () => {
		const msg = state.commit ? fmt.body(state.commit.message) : "";
		const desc = h(
			"div.comment",
			{},
			h("div.comment-head", {}, avatar(state.commit ? state.commit.author.name : opener() || "?", "sm"), h("span.who", {}, state.commit ? state.commit.author.name : opener()), h("span.muted", {}, "wrote"), state.commit ? time(state.commit.author.when) : null, h("span.spacer"), newest ? h("a.hash", { href: `${base}/commit/${newest.commit}` }, fmt.short(newest.commit, 8)) : null),
			h("div.comment-body", {}, msg ? markdown(msg) : h("p.muted", {}, "No description: the commit message has only a title.")),
		);
		// the timeline: versions, votes, landing and the general comments
		const items = [];
		for (const v of state.versions) items.push({ at: fmt.toMs(v.time), el: tl(v.version === 1 ? "upload" : "refresh", "blue", [h("b", {}, v.pushed_by || "someone"), v.version === 1 ? " pushed the first version " : ` pushed version ${v.version} `, h("a.hash", { href: `${base}/commit/${v.commit}` }, fmt.short(v.commit, 8)), v.version > 1 ? [" · ", h("a", { href: `${base}/change/${ch}/files?version=${v.version}&against=previous` }, "compare with v", String(v.version - 1))] : null], v.time) });
		for (const v of state.votes.votes || []) items.push({ at: fmt.toMs(v.at), el: tl(v.vote === "approve" ? "check" : "alert", v.vote === "approve" ? "green" : "amber", [h("b", {}, v.name), v.vote === "approve" ? " approved" : " requested changes", ` on v${v.version}`], v.at) });
		const rv = state.review;
		if (rv && rv.state === "landed") items.push({ at: fmt.toMs(rv.landed_at), el: tl("landed", "purple", [h("b", {}, rv.landed_by_name || "someone"), " landed it as ", h("a.hash", { href: `${base}/commit/${rv.landed_commit}` }, fmt.short(rv.landed_commit, 8))], rv.landed_at) });
		for (const t of threads(state.comments)) items.push({ at: fmt.toMs(t.root.created_at), el: h("div.tl-item", {}, h("div.tl-icon", {}, icon("comment")), thread(t, commentAct)) });
		items.sort((a, b) => a.at - b.at);
		const box = me
			? composer({ placeholder: "Leave a comment on the change", onSubmit: (text) => addComment(text) })
			: h("div.callout", {}, icon("login"), h("span", {}, h("a", { href: "/login?next=" + encodeURIComponent(location.pathname) }, "Sign in"), " to comment, vote or land."));
		content.replaceChildren(desc, h("div.timeline", { style: { "margin-top": "16px" } }, items.map((i) => i.el)), mergeBox(), h("div", { style: { "margin-top": "20px" } }, box));
	};

	const tl = (ic, color, text, at) => h("div.tl-item", {}, h("div", { class: ["tl-icon", color] }, icon(ic)), h("div.tl-body", {}, text, " ", h("span.faint", {}, time(at))));

	const checkRow = (c) =>
		h(
			"div.check-row",
			{},
			h("span", { class: c.state === "success" ? "green" : c.state === "pending" ? "amber" : "red" }, icon(c.state === "success" ? "checkCircle" : c.state === "pending" ? "clock" : "xCircle", "sm")),
			h("span.name", {}, c.name),
			c.description ? h("span.desc.ellipsis", {}, c.description) : null,
			h("span.spacer"),
			c.url ? h("a.small", { href: c.url, target: "_blank", rel: "noopener noreferrer" }, "Details ", icon("external", "sm")) : null,
			h("span.tiny.faint", {}, time(c.updated_at)),
		);

	// ---- files ----

	let filesRedraw = null;
	const drawFiles = () => {
		const versionN = Number(ctx.query.version) || newest.version;
		let against = ctx.query.against === "previous" && versionN > 1 ? "previous" : "base";
		let split = store.pref("diff.split", false);
		const out = h("div", {}, skeleton(8));
		const vpick = h(
			"div.version-picker",
			{},
			h("span.small.muted", { style: { "margin-right": "4px" } }, "Version"),
			state.versions.map((v) =>
				h(
					"button",
					{
						type: "button",
						class: v.version === versionN ? "to" : against === "previous" && v.version === versionN - 1 ? "from" : "",
						title: `v${v.version} · ${fmt.short(v.commit, 8)}`,
						onclick: () => {
							setQuery({ version: v.version === newest.version ? "" : v.version, against: against === "previous" && v.version > 1 ? "previous" : "" });
							ctx.query.version = String(v.version);
							ctx.query.against = against;
							drawFiles();
						},
					},
					"v" + v.version,
				),
			),
		);
		const againstPick = segmented(
			[
				{ value: "base", label: "Against base" },
				{ value: "previous", label: "Against previous version" },
			],
			against,
			(v) => {
				if (v === "previous" && versionN < 2) {
					toast("The first version has no version before it.", "info");
					return;
				}
				ctx.query.against = v;
				setQuery({ against: v === "base" ? "" : v });
				drawFiles();
			},
		);
		const splitPick = segmented(
			[
				{ value: false, icon: "unified", title: "Unified" },
				{ value: true, icon: "split", title: "Split" },
			],
			split,
			(v) => {
				split = v;
				store.setPref("diff.split", v);
				drawFiles();
			},
		);
		content.replaceChildren(h("div.row.row-wrap.mb-4", {}, vpick, h("span.spacer"), againstPick, splitPick), out);
		let tries = 0;
		const fetchDiff = async () => {
			let d;
			try {
				d = await api.request("GET", C(`/diffs/${against}`), { params: { version: versionN }, accept202: true });
			} catch (err) {
				out.replaceChildren(errorBox(err));
				return;
			}
			if (!ctx.alive()) return;
			if (d && d.pending) {
				out.replaceChildren(h("div.loading", {}, h("span.spinner"), "Computing the semantic diff…"));
				if (tries++ < 60) setTimeout(fetchDiff, 1500);
				return;
			}
			renderDiff(out, d, versionN, split);
		};
		fetchDiff();
		filesRedraw = () => drawFiles();
	};

	const renderDiff = (out, d, versionN, split) => {
		const files = d.files || [];
		const isNewest = versionN === newest.version;
		const anchored = state.comments.filter((c) => c.file && !c.outdated);
		const pendingBox = { key: "", el: null };
		const sum = h(
			"div.box.mb-4",
			{},
			h("div.box-head", {}, d.kind === "semantic" ? badge("Semantic", "purple", "sparkle") : badge("Lines", "outline"), h("span.small.muted", {}, fmt.plural(files.length, "file"), " · ", d.against === "previous" ? `v${versionN - 1} → v${versionN}` : `base → v${versionN}`, " · ", h("a.hash", { href: `${base}/commit/${d.from}` }, fmt.short(d.from, 8)), " → ", h("a.hash", { href: `${base}/commit/${d.to}` }, fmt.short(d.to, 8))), d.reason ? h("span.small.amber", {}, d.reason) : null),
			files.length
				? files.map((f) =>
						h(
							"a.box-row.plain",
							{ href: "#" + encodeURIComponent(f.path), style: { "min-height": "34px", padding: "6px 16px" } },
							icon("fileCode", "sm"),
							h("span.mono.small.grow.ellipsis", {}, f.old_path && f.old_path !== f.path ? `${f.old_path} → ${f.path}` : f.path),
							(f.decls || []).length ? h("span.row", { style: { gap: "4px" } }, Object.entries(countKinds(f.decls)).map(([k, n]) => badge(`${n} ${k}`, DECL_COLOR[k] || ""))) : null,
						),
					)
				: h("div.box-empty", {}, "No files changed."),
		);
		const blocks = files.map((f) => {
			const lang = fmt.language(f.path);
			const fileComments = anchored.filter((c) => c.file === f.path);
			return h(
				"div",
				{ id: encodeURIComponent(f.path) },
				fileBlock({
					path: f.path,
					oldPath: f.old_path,
					kind: "",
					badge: fileComments.length ? badge(fmt.plural(fileComments.length, "comment"), "accent", "comment") : null,
					actions: [h("a.btn.sm.ghost", { href: `${base}/blob/${d.to}/${api.enc(f.path)}`, title: "View the file at this version" }, icon("eye", "sm"))],
					build: () => {
						if ((f.decls || []).length) {
							return h(
								"div",
								{},
								f.decls.map((dc) => {
									const declComments = fileComments.filter((c) => c.decl === dc.key);
									const hk = df.parseUnified(dc.diff);
									const rowExtra = (row) => {
										if (!row || !row.b) return null;
										const off = row.b - 1;
										const here = declComments.filter((c) => c.line_offset === off && !c.parent);
										const key = `${f.path}|${dc.key}|${off}`;
										const parts = threads(state.comments.filter((c) => c.file === f.path && c.decl === dc.key && (c.line_offset === off || c.parent))).filter((t) => here.includes(t.root));
										if (!parts.length && pendingBox.key !== key) return null;
										return h("div.col", { style: { gap: "10px", "max-width": "860px" } }, parts.map((t) => thread(t, commentAct, { showAnchor: false })), pendingBox.key === key ? pendingBox.el : null);
									};
									const onComment =
										me && isNewest && dc.change !== "removed"
											? (row) => {
													if (!row.b) {
														toast("Comment on a line of the new version.", "info");
														return;
													}
													const off = row.b - 1;
													pendingBox.key = `${f.path}|${dc.key}|${off}`;
													pendingBox.el = composer({
														placeholder: `Comment on ${dc.key}, line ${off + 1}`,
														submitLabel: "Comment",
														compact: true,
														onCancel: () => {
															pendingBox.key = "";
															redrawDecl();
														},
														onSubmit: async (text) => {
															pendingBox.key = "";
															await addComment(text, { file: f.path, decl: dc.key, line_offset: off, version: versionN });
														},
													});
													redrawDecl();
													setTimeout(() => pendingBox.el && pendingBox.el.focus(), 20);
												}
											: null;
									const wrap = h("div");
									const redrawDecl = () => wrap.replaceChildren(hk.length ? hunkTable(hk, { lang, split, onComment, rowExtra }) : h("div.box-empty.small", {}, dc.change === "moved" ? "Moved without changes." : "No line changes."));
									redrawDecl();
									return h(
										"div.decl-block",
										{},
										h("div.decl-head", {}, badge(dc.change, DECL_COLOR[dc.change] || ""), dc.old_key && dc.old_key !== dc.key ? [h("span.old", {}, dc.old_key), icon("arrowRight", "sm")] : null, h("span.key", {}, dc.key), declComments.length ? badge(String(declComments.length), "accent", "comment") : null),
										h("div", { style: { overflow: "auto" } }, wrap),
									);
								}),
							);
						}
						const hk = df.parseUnified(f.diff || "");
						return hk.length ? h("div", { style: { overflow: "auto" } }, hunkTable(hk, { lang, split })) : h("div.box-empty", {}, "No line changes.");
					},
				}),
			);
		});
		const general = !isNewest ? callout("info", "You're looking at an older version; comments go on the newest.") : null;
		out.replaceChildren(general || "", sum, ...blocks);
	};

	const countKinds = (decls) => {
		const m = {};
		for (const d of decls) m[d.change] = (m[d.change] || 0) + 1;
		return m;
	};

	// ---- checks, bench, behaviour ----

	const drawChecks = () => {
		const rv = state.review;
		const all = rv ? rv.checks : [];
		const byVersion = new Map();
		for (const c of all) {
			if (!byVersion.has(c.version)) byVersion.set(c.version, []);
			byVersion.get(c.version).push(c);
		}
		const req = rv ? rv.required || [] : [];
		content.replaceChildren(
			req.length ? callout("info", "Required to land: ", req.map((n, i) => [i ? ", " : "", h("b", {}, n)])) : null,
			h("div", { style: { height: "12px" } }),
			byVersion.size
				? [...byVersion.entries()]
						.sort((a, b) => b[0] - a[0])
						.map(([v, cs]) => h("div.box.mb-4", {}, h("div.box-head", {}, h("b", {}, "Version ", String(v)), v === newest.version ? badge("newest", "accent") : null), h("div", { style: { padding: "6px 16px" } }, cs.map(checkRow))))
				: empty("checkCircle", "No checks yet", "CI posts checks with a signed request to the checks API; required checks are set in the repository's review settings."),
		);
	};

	const drawBench = async () => {
		content.replaceChildren(skeleton(6));
		try {
			const v = await api.get(C("/bench"));
			if (!ctx.alive()) return;
			const rows = v.benchmarks || [];
			content.replaceChildren(
				h(
					"div.box.mb-4",
					{},
					h("div.box-head", {}, h("b", {}, "Benchmark verdict"), v.state ? badge(v.state, v.state === "success" ? "green" : v.state === "failure" ? "red" : "amber") : null, h("span.spacer"), h("span.small.muted", {}, `Threshold ${fmt.percent(v.threshold || 0)} slower · base `, h("a.hash", { href: `${base}/commit/${v.base}` }, fmt.short(v.base, 8)))),
					rows.length
						? h(
								"table.table",
								{},
								h("thead", {}, h("tr", {}, ["Benchmark", "Machine", "Base", "This change", "Change", ""].map((t, i) => h("th", { class: i >= 2 && i <= 4 ? "num" : "" }, t)))),
								h(
									"tbody",
									{},
									rows.map((b) =>
										h(
											"tr",
											{},
											h("td", {}, h("a", { href: `${base}/bench/${encodeURIComponent(b.name)}` }, b.name)),
											h("td.small.muted", {}, [b.machine, b.arch, b.cpu].filter(Boolean).join(" · ")),
											h("td.num.mono", {}, fmt.metric(b.base, b.unit)),
											h("td.num.mono", {}, fmt.metric(b.value, b.unit)),
											h("td.num.mono", { class: b.slower > 0.001 ? "red" : b.slower < -0.001 ? "green" : "muted" }, fmt.percent(b.slower, true)),
											h("td", {}, b.regressed ? badge("regressed", "red", "alert") : null),
										),
									),
								),
							)
						: h("div.box-empty", {}, "No benchmark results for this version and its base on the same Linux machine yet. Record them with ", h("code", {}, "tit bench record"), "."),
				),
			);
		} catch (err) {
			content.replaceChildren(err.status === 404 ? empty("gauge", "No benchmarks", "This change has no benchmark results.") : errorBox(err));
		}
	};

	const drawBehaviour = async () => {
		content.replaceChildren(skeleton(6));
		const ask = me ? btn("Run behaviour", { icon: "play", primary: true }) : null;
		if (ask)
			ask.onclick = () =>
				busy(ask, async () => {
					const r = await api.post(C("/behaviour"), { version: newest.version });
					toast(`Run ${r.state || "queued"} for v${r.version}`);
					setTimeout(drawBehaviour, 1200);
				});
		try {
			const b = await api.get(C("/behaviour"));
			if (!ctx.alive()) return;
			const run = b.run || {};
			const groups = b.groups || [];
			content.replaceChildren(
				h(
					"div.box.mb-4",
					{},
					h("div.box-head", {}, h("b", {}, "Behaviour run"), badge(run.state || "?", run.state === "done" ? "green" : run.state === "failed" ? "red" : "amber"), h("span.small.muted", {}, `v${run.version} · ${fmt.plural(run.capsules || 0, "capsule")} replayed`), h("span.spacer"), ask),
					run.reason ? h("div", { style: { padding: "10px 16px" } }, callout("warn", run.reason)) : null,
					groups.length
						? h(
								"table.table",
								{},
								h("thead", {}, h("tr", {}, ["Outcome", "What", "Effect", "Capsules"].map((t) => h("th", {}, t)))),
								h(
									"tbody",
									{},
									groups.map((g) =>
										h(
											"tr",
											{},
											h("td", {}, badge(g.outcome, g.outcome === "same" ? "" : g.outcome === "fixed" ? "green" : g.outcome === "broken" || g.outcome === "new_panic" ? "red" : "amber")),
											h("td", {}, h("div.strong", {}, g.label || g.key), h("div.tiny.muted.mono", {}, g.key)),
											h("td.mono", {}, String(g.effect)),
											h("td", {}, String(g.count)),
										),
									),
								),
							)
						: h("div.box-empty", {}, run.state === "done" ? "Every replayed request behaved the same." : "Nothing to show yet."),
				),
			);
		} catch (err) {
			content.replaceChildren(err.status === 404 ? empty("runner", "No behaviour run yet", "A run replays recorded production requests against this version and its base, and groups what changed.", ask) : err.status === 403 ? empty("lock", "No access to replays", err.message) : errorBox(err));
		}
	};

	drawHead();
	drawTabs();
	drawSidebar();
	if (tab === "files") drawFiles();
	else if (tab === "checks") drawChecks();
	else if (tab === "bench") drawBench();
	else if (tab === "behaviour") drawBehaviour();
	else drawConversation();
	if (ctx.hash && ctx.hash.startsWith("c")) setTimeout(() => document.getElementById(ctx.hash)?.scrollIntoView({ block: "center" }), 50);

	// live: reload when anything happens to the change
	const again = debounce(() => reload().catch(() => {}), 400);
	ctx.cleanup(live.follow(`change:${repo.owner}/${repo.name}/${ch}`, () => again()));
}
