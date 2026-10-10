// Benchmarks: every benchmark the repository records, and one benchmark's results over time, a line per machine.
// Development machines (macOS) are shown apart and never as results: Linux decides.

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as fmt from "../lib/format.js";
import { navigate } from "../lib/router.js";
import { frame } from "./repo.js";
import { line, spark, color } from "../ui/chart.js";
import { badge, callout, empty, errorBox, segmented, skeleton, time } from "../ui/kit.js";

const machineLabel = (s) => s.machine || [[s.os, s.arch].filter(Boolean).join("/"), s.cpu].filter(Boolean).join(" · ") || "unknown machine";

export async function render(ctx) {
	const { repo, body, base } = await frame(ctx, "bench");
	const R = api.R(repo.owner, repo.name);
	const name = ctx.params.name;
	if (name) return one(ctx, repo, body, base, R, name);
	ctx.title("Benchmarks", `${repo.owner}/${repo.name}`);
	const grid = h("div", {}, skeleton(6));
	const filter = h("input.input", { type: "search", placeholder: "Filter benchmarks", style: { "max-width": "280px" } });
	body.append(h("div.page-head", {}, h("div", {}, h("h2", {}, "Benchmarks"), h("p.sub", {}, "Results recorded with ", h("code", {}, "tit bench record"), ", per machine. Only Linux results count.")), filter), grid);
	let list;
	try {
		list = (await api.get(R("/bench"))).benchmarks || [];
	} catch (err) {
		grid.replaceChildren(errorBox(err));
		return;
	}
	if (!ctx.alive()) return;
	if (!list.length) {
		grid.replaceChildren(h("div.box", {}, empty("gauge", "No benchmarks yet", ["Record results on a Linux machine with ", h("code", {}, "tit bench record"), "; each review then shows how its change moves them."])));
		filter.remove();
		return;
	}
	const cards = list.map((b) => {
		const sp = h("div", { style: { height: "40px" } });
		const card = h(
			"a.bench-card.plain",
			{ href: `${base}/bench/${encodeURIComponent(b.name)}`, "data-name": b.name.toLowerCase() },
			h("div.row", {}, icon("gauge", "sm"), h("span.name.grow", {}, b.name)),
			sp,
			h("div.row.small.muted", { style: { gap: "10px" } }, h("span", {}, fmt.plural(b.results, "result")), b.results !== b.linux_results ? h("span", {}, `${b.linux_results} on Linux`) : null, h("span.spacer"), b.newest_at ? time(b.newest_at) : null),
		);
		// a sparkline of the first Linux machine's newest points, fetched lazily
		const io = new IntersectionObserver((es) => {
			if (!es.some((e) => e.isIntersecting)) return;
			io.disconnect();
			api.get(R(`/bench/${api.enc(b.name)}`)).then((d) => {
				const s = (d.series || []).find((x) => !x.development) || (d.series || [])[0];
				if (s && s.points.length) sp.replaceChildren(spark(s.points.slice(-40).map((p) => p.value), { width: 280, height: 40 }));
			}, () => {});
		});
		io.observe(card);
		ctx.cleanup(() => io.disconnect());
		return card;
	});
	grid.replaceChildren(h("div.bench-grid", {}, cards));
	filter.oninput = () => {
		const q = filter.value.trim().toLowerCase();
		for (const c of cards) c.hidden = q && !c.dataset.name.includes(q);
	};
}

async function one(ctx, repo, body, base, R, name) {
	ctx.title(name, `${repo.owner}/${repo.name}`);
	body.append(skeleton(8));
	let d;
	try {
		d = await api.get(R(`/bench/${api.enc(name)}`));
	} catch (err) {
		if (err.status === 404) return ctx.notFound(`There is no benchmark ${name}.`);
		body.replaceChildren(errorBox(err));
		return;
	}
	if (!ctx.alive()) return;
	const series = d.series || [];
	const linux = series.filter((s) => !s.development);
	const dev = series.filter((s) => s.development);
	const chartBox = h("div");
	const tableBox = h("div");
	let showDev = false;
	// by time when the results span more than a day, else in the order they were recorded
	const ats = series.flatMap((x) => x.points.map((p) => fmt.toMs(p.at)));
	let byTime = ats.length > 1 && Math.max(...ats) - Math.min(...ats) > 86400000;
	const draw = () => {
		const shown = showDev ? series : linux;
		const units = [...new Set(shown.map((s) => s.unit))];
		const lines = shown.map((s, i) => ({
			label: machineLabel(s),
			color: s.development ? "var(--fg-faint)" : color(i),
			points: s.points.map((p, j) => ({
				x: byTime ? fmt.toMs(p.at) : j,
				y: p.value,
				tip: [fmt.metric(p.value, s.unit), machineLabel(s), fmt.short(p.commit, 8) + (p.change ? " · " + fmt.shortChange(p.change) : ""), p.kernel ? "kernel " + p.kernel : "", fmt.date(p.at, true)].filter(Boolean),
				onclick: () => navigate(p.change ? `${base}/change/${p.change}` : `${base}/commit/${p.commit}`),
			})),
		}));
		chartBox.replaceChildren(
			shown.length
				? h(
						"div.card",
						{},
						h(
							"div.card-body",
							{},
							h("div.legend.mb-4", {}, lines.map((l) => h("span", {}, h("span.sw", { style: { background: l.color } }), l.label))),
							line(lines, { unit: units.length === 1 ? units[0] : "", xLabel: byTime ? (x) => fmt.date(x) : (x) => "#" + Math.round(x + 1) }),
							h("p.tiny.muted", { style: { margin: "8px 0 0" } }, "Click a point to open its change or commit."),
						),
					)
				: h("div.box", {}, empty("gauge", "No Linux results", "Only development machines recorded this benchmark. Development numbers are not results.")),
		);
		tableBox.replaceChildren(
			h(
				"div.box",
				{},
				h("div.box-head", {}, h("b", {}, "Newest results")),
				h(
					"table.table",
					{},
					h("thead", {}, h("tr", {}, ["Machine", "Kernel", "Commit", "Value", "Points", "When"].map((t, i) => h("th", { class: i === 3 || i === 4 ? "num" : "" }, t)))),
					h(
						"tbody",
						{},
						shown.map((s) => {
							const p = s.points[s.points.length - 1];
							return h(
								"tr",
								{},
								h("td", {}, h("b", {}, [s.os, s.arch].filter(Boolean).join("/") || "?"), h("div.tiny.muted", {}, s.cpu || s.machine), s.development ? badge("development", "amber outline") : null),
								h("td.small.mono", {}, p ? p.kernel : ""),
								h("td", {}, p ? h("a.hash", { href: `${base}/commit/${p.commit}` }, fmt.short(p.commit, 8)) : null),
								h("td.num.mono", {}, p ? fmt.metric(p.value, s.unit) : ""),
								h("td.num", {}, String(s.points.length)),
								h("td.small.muted", {}, p ? time(p.at) : null),
							);
						}),
					),
				),
			),
		);
	};
	body.replaceChildren(
		h("div.crumbs.mb-4", {}, h("a", { href: `${base}/bench` }, "Benchmarks"), h("span.sep", {}, "/"), h("span.mono", {}, name)),
		h(
			"div.page-head",
			{},
			h("div", {}, h("h2", { style: { "font-family": "var(--font-mono)" } }, name), h("p.sub", {}, fmt.plural(linux.length, "Linux machine"), dev.length ? ` · ${fmt.plural(dev.length, "development machine")}` : "")),
			h(
				"div.row",
				{},
				segmented(
					[
						{ value: true, label: "By time" },
						{ value: false, label: "By result" },
					],
					byTime,
					(v) => ((byTime = v), draw()),
				),
				dev.length
					? segmented(
							[
								{ value: false, label: "Linux" },
								{ value: true, label: "All machines" },
							],
							showDev,
							(v) => ((showDev = v), draw()),
						)
					: null,
			),
		),
		dev.length ? callout("info", "Development machines (macOS) are shown for reference only: Linux numbers decide.") : null,
		h("div", { style: { height: "12px" } }),
		chartBox,
		h("div", { style: { height: "20px" } }),
		tableBox,
	);
	draw();
}
