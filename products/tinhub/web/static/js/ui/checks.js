// Replay checks (design/tinhub.md §12.1): pick a branch, tag, change or commit, and tinhub's runner builds it and
// replays a failure group's capsules (or one of them) against it on the server. Each capsule gets a verdict with the
// reason for it: passed, diverged at effect N, panicked, still failing, timed out or skipped.

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as fmt from "../lib/format.js";
import { ORDER, worst } from "../lib/verdicts.js";
import { refsOf } from "./refs.js";
import { badge, btn, busy, callout, empty, errorBox, skeleton, time } from "./kit.js";

// VERDICTS is how each verdict shows, in the order a check lists them.
export const VERDICTS = {
	passed: { label: "Passed", color: "green", icon: "checkCircle", means: "The build made exactly the calls production made, in the same order, did not panic and answered below 500. Every answer it read was the real one from the incident, so this is the fix working on what actually happened." },
	diverged: { label: "Diverged", color: "amber", icon: "split", means: "The build made a different call than production at some effect (or one more, or one fewer). A replay can only answer calls production made, so past that point the result is unknown. Not a failure of the fix, but not proof of it either." },
	panicked: { label: "Panicked", color: "red", icon: "bug", means: "The same calls as production, and the build still panics (or gives no response). The fix does not reach this request." },
	failing: { label: "Still fails", color: "red", icon: "xCircle", means: "The same calls and no panic, but a 5xx answer. When the capsule recorded a dependency failing, replay serves that same failure, so an error can be the right answer." },
	timeout: { label: "Timed out", color: "amber", icon: "clock", means: "The replay did not finish. Nothing outside the build is waited on in a replay, so the build loops or waits on something nothing wakes." },
	skipped: { label: "Skipped", color: "outline", icon: "ban", means: "The capsule could not be replayed, usually because it was not sealed for the runner's key." },
};

// OUTCOME is a whole check's outcome in one word, by its worst verdict.
const OUTCOME = { passed: "Fixed", panicked: "Still panics", failing: "Still fails", timeout: "Timed out", diverged: "Diverged", skipped: "Skipped" };

function verdictBadge(v, n) {
	const d = VERDICTS[v] || { label: v, color: "outline", icon: "dot" };
	return badge(n === undefined ? d.label : `${n} ${d.label.toLowerCase()}`, d.color, d.icon);
}

// outcome is a check's state or outcome as one badge: the first thing to read on its row.
export function outcome(k) {
	if (k.state === "queued") return badge("Queued", "outline", "clock");
	if (k.state === "running") return badge(h("span.row", { style: { gap: "6px" } }, h("span.spinner", { style: { width: "10px", height: "10px" } }), "Running"), "blue");
	if (k.state === "failed") return badge("Could not run", "red", "alert");
	if (k.state === "skipped") return badge("Skipped", "outline", "ban");
	const v = worst(k.counts);
	if (!v) return badge("No capsules", "outline");
	const d = VERDICTS[v];
	return badge(OUTCOME[v], d.color, d.icon);
}

// tally is how many of a done check's capsules passed, in words ("2 of 5 capsules pass").
function tally(k) {
	if (k.state !== "done") return "";
	const total = (k.counts || []).reduce((n, c) => n + c.count, 0);
	if (!total) return "";
	const passed = ((k.counts || []).find((c) => c.verdict === "passed") || { count: 0 }).count;
	if (total === 1) return "";
	if (passed === total) return `all ${total} capsules pass`;
	return `${passed} of ${fmt.plural(total, "capsule")} pass`;
}

// headline is the check's outcome in one sentence.
function headline(k) {
	if (k.state === "queued") return "Waiting for the runner.";
	if (k.state === "running") return "Building the revision and replaying the capsules.";
	if (k.state !== "done") return k.reason || k.state;
	return {
		passed: "Handles the recorded failure: the same calls as production, no panic, no 5xx.",
		panicked: "The panic is still there: the fix does not reach this request.",
		failing: "Answers 5xx on the same calls production made.",
		diverged: "Makes different calls than production did, so the replay cannot judge the answer.",
		timeout: "The replay did not finish.",
		skipped: "The capsules could not be opened by the runner.",
	}[worst(k.counts)] || k.reason || "";
}

// resultRow is one capsule's verdict: a line, with the explanation one click away.
function resultRow(r) {
	const d = VERDICTS[r.verdict] || VERDICTS.skipped;
	const facts = [
		h("span", {}, "recorded ", h("b", {}, r.recorded ? String(r.recorded) : "none")),
		h("span", {}, "now ", h("b", {}, r.status ? String(r.status) : r.verdict === "skipped" ? "not run" : "no response")),
		r.effect >= 0 ? h("span", {}, "first different call: effect ", h("b", {}, String(r.effect)), ` (${r.got}${r.want ? ` vs ${r.want}` : ", nothing recorded"})`) : null,
		r.left > 0 ? h("span", {}, fmt.plural(r.left, "recorded call") + " not made") : null,
		r.panic_in ? h("span", {}, "panicked in ", h("span.mono", {}, r.panic_in)) : null,
	];
	return h(
		"details.rcheck-result",
		{},
		h(
			"summary",
			{},
			verdictBadge(r.verdict),
			h("span.rcheck-label", {}, r.label),
			h("span.spacer"),
			r.route ? h("span.mono.small.muted", {}, r.route) : null,
			h("span.mono.small.muted", { title: r.capsule }, fmt.short(r.capsule, 8)),
		),
		h("p.rcheck-why", {}, r.detail || d.means),
		h("div.row.tiny.muted", { style: { gap: "12px", "flex-wrap": "wrap" } }, facts),
	);
}

// checksPanel is the group page's "Check a fix" panel. capsules are the group's, for checking one of them; the panel's
// pick(capsuleId) preselects one and focuses the revision field.
export function checksPanel(ctx, repo, base, R, group, capsules) {
	const target = h("input.input.mono", { placeholder: "Branch, tag, change or commit (fix/declined-cart)", list: "check-targets", "aria-label": "Revision to check", style: { "min-width": "240px", flex: "1" } });
	const targets = h("datalist", { id: "check-targets" });
	const which = h(
		"select.select",
		{ "aria-label": "Capsules to replay", style: { width: "auto" } },
		h("option", { value: "" }, capsules.length === 1 ? "Its capsule" : `All ${Math.min(capsules.length, 20)} capsules`),
		capsules.map((c) => h("option", { value: c.id }, `Capsule ${fmt.short(c.id, 10)}${c.route ? " · " + c.route : ""}`)),
	);
	const run = btn("Replay against it", { primary: true, icon: "play" });
	const notice = h("div");
	const list = h("div", {}, skeleton(3));
	const open = new Set(ctx.query.check ? [Number(ctx.query.check)] : []);
	let timer = 0;
	let showOlder = false;

	refsOf(repo.owner, repo.name).then(
		(refs) => {
			for (const r of refs) targets.append(h("option", { value: r.short }, r.kind === "tag" ? "tag" : r.short === repo.default_branch ? "default branch" : "branch"));
			if (!target.value) target.placeholder = `Branch, tag, change or commit (${(refs.find((r) => r.kind === "branch" && r.short !== repo.default_branch) || refs[0] || { short: "main" }).short})`;
		},
		() => {},
	);

	const detail = async (k, box) => {
		box.replaceChildren(skeleton(2));
		try {
			const d = await api.get(R(`/replay/checks/${k.id}`));
			if (!ctx.alive()) return;
			const results = d.results || [];
			box.replaceChildren(
				k.reason ? callout(k.state === "done" ? "info" : "warn", h("pre.rcheck-reason", {}, k.reason)) : null,
				results.length ? results.map(resultRow) : k.state === "done" ? h("p.small.muted", {}, "No capsule was replayed.") : null,
			);
		} catch (err) {
			box.replaceChildren(errorBox(err));
		}
	};

	const row = (k, older) => {
		const box = h("div.rcheck-detail", { hidden: !open.has(k.id) });
		const chevron = h("span.rcheck-chevron", {}, icon("chevronRight", "sm"));
		const toggle = h(
			"button.rcheck-row",
			{ type: "button", "aria-expanded": String(open.has(k.id)) },
			chevron,
			h("div.rcheck-outcome", {}, outcome(k)),
			h(
				"div.grow",
				{ style: { "min-width": "0" } },
				h("div.row", { style: { gap: "8px", "flex-wrap": "wrap" } }, h("b.mono", {}, k.target), h("span.mono.small.muted", {}, fmt.short(k.commit, 8)), k.capsule ? h("span.small.muted", {}, "one capsule") : null),
				h("div.small.muted.rcheck-headline", {}, [tally(k), headline(k)].filter(Boolean).join(". ").replace(/^./, (c) => c.toUpperCase())),
			),
			h("div.tiny.muted.nowrap", {}, time(k.created_at)),
		);
		const show = (now) => {
			if (now) open.add(k.id);
			else open.delete(k.id);
			toggle.setAttribute("aria-expanded", String(now));
			box.hidden = !now;
			if (now) detail(k, box);
		};
		toggle.onclick = () => show(!open.has(k.id));
		if (open.has(k.id)) detail(k, box);
		return h(older ? "div.rcheck.rcheck-older" : "div.rcheck", { id: `check-${k.id}` }, toggle, box);
	};

	// latestFirst keeps each revision's newest check on top and puts its earlier checks behind one toggle.
	const listOf = (checks) => {
		const seen = new Set();
		const latest = [];
		const older = [];
		for (const k of checks) {
			if (seen.has(k.target)) older.push(k);
			else {
				seen.add(k.target);
				latest.push(k);
			}
		}
		const rows = latest.map((k) => row(k, false));
		if (!older.length) return rows;
		const more = h("details.rcheck-more", { open: showOlder }, h("summary.small.muted", {}, `${fmt.plural(older.length, "earlier check")} of the same revisions`), older.map((k) => row(k, true)));
		more.addEventListener("toggle", () => (showOlder = more.open));
		return [...rows, more];
	};

	// older holds the pages read with "Show older checks", and cursor the next page's before (0 when none is left)
	let older = [];
	let cursor = 0;
	const page = (before) => api.get(R("/replay/checks"), { group: group.id, limit: 30, before: before || undefined });

	const render = (first) => {
		const ids = new Set(first.map((k) => k.id));
		const checks = [...first, ...older.filter((k) => !ids.has(k.id))];
		const more = cursor ? btn("Show older checks", { sm: true }) : null;
		if (more)
			more.onclick = () =>
				busy(more, async () => {
					const d = await page(cursor);
					if (!ctx.alive()) return;
					older = [...older, ...(d.checks || [])];
					cursor = d.next || 0;
					render(first);
				});
		list.replaceChildren(
			checks.length ? listOf(checks) : empty("runner", "No checks yet", "Push your fix to a branch, then replay this failure against it. A check tells you whether the fix handles the request that failed in production, before you merge it."),
			more ? h("div.rcheck-more-pages", {}, more) : null,
		);
	};

	const load = async () => {
		clearTimeout(timer);
		let d;
		try {
			d = await page(0);
		} catch (err) {
			list.replaceChildren(errorBox(err));
			return;
		}
		if (!ctx.alive()) return;
		notice.replaceChildren(
			d.opted_in ? null : callout("warn", h("b", {}, "The runner is off for this repository. "), `Checks build and replay on tinhub's runner, which only replays capsules of owners that opted in. An owner of ${repo.owner} turns it on with `, h("code", {}, `PUT /api/v1/orgs/${repo.owner}/runner`), ", then adds the runner's public key to the recording server's ", h("code", {}, "TIN_REPLAY_RECIPIENTS"), "."),
		);
		const checks = d.checks || [];
		if (!older.length) cursor = d.next || 0;
		render(checks);
		if (checks.some((k) => k.state === "queued" || k.state === "running")) timer = setTimeout(() => ctx.alive() && load(), 2500);
	};

	run.onclick = () =>
		busy(run, async () => {
			const t = target.value.trim();
			if (!t) {
				target.focus();
				throw new Error("Pick a branch, tag, change or commit to check.");
			}
			const k = await api.post(R("/replay/checks"), { target: t, group: group.id, capsule: which.value });
			open.add(k.id);
			await load();
		});
	target.addEventListener("keydown", (e) => {
		if (e.key === "Enter") run.click();
	});

	const legend = h(
		"details.rcheck-legend",
		{},
		h("summary.small", {}, "What the verdicts mean"),
		h("dl", {}, ORDER.map((v) => [h("dt", {}, verdictBadge(v)), h("dd.small", {}, VERDICTS[v].means)])),
	);

	const panel = h(
		"div.box.mb-4.checks-panel",
		{},
		h("div.box-head", {}, icon("runner", "sm"), h("b", {}, "Check a fix"), h("span.spacer"), h("span.small.muted", {}, "Built and replayed on the server, in a sandbox")),
		h(
			"div.box-body",
			{},
			h("p.small.muted", { style: { margin: "0 0 12px" } }, "Pick the branch with your fix. tinhub builds it and replays the failed requests against it, with every Redis, HTTP and SQL answer exactly as production saw it."),
			notice,
			h("div.row", { style: { gap: "8px", "flex-wrap": "wrap" } }, target, targets, which, run),
			legend,
		),
		list,
	);
	panel.pick = (capsuleId) => {
		which.value = capsuleId;
		panel.scrollIntoView({ behavior: "smooth", block: "start" });
		target.focus();
	};
	load();
	return panel;
}

