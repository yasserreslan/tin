// Create a repository, for yourself or an org you own.

import { h } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import * as store from "../lib/store.js";
import { navigate } from "../lib/router.js";
import { input, textarea, select } from "../ui/forms.js";
import { btn, busy, field, loginHref } from "../ui/kit.js";

// as accounts.ValidRepoName
const valid = (v) => /^[A-Za-z0-9._-]{1,100}$/.test(v) && v !== "." && v !== ".." && !v.endsWith(".tit");

export async function render(ctx) {
	const me = store.session.user;
	if (!me) return navigate(loginHref(), { replace: true });
	ctx.title("New repository");
	const owners = [me.name, ...(me.orgs || []).filter((o) => o.role === "owner").map((o) => o.name)];
	const owner = select(owners, owners.includes(ctx.query.owner) ? ctx.query.owner : me.name, { style: { width: "auto", "min-width": "160px" } });
	const name = input("", { placeholder: "my-service", autocomplete: "off", spellcheck: "false", autofocus: true });
	const desc = textarea("", { rows: 2, placeholder: "Optional", maxlength: 350 });
	let vis = "public";
	const visCard = (v, ic, title, text) => {
		const radio = h("input", { type: "radio", name: "vis", value: v, checked: v === vis });
		const card = h("label.radio-card", { class: v === vis ? "on" : "" }, radio, h("div", {}, h("div.row", { style: { gap: "6px" } }, icon(ic, "sm"), h("b", {}, title)), h("div.small.muted", {}, text)));
		radio.onchange = () => {
			vis = v;
			for (const c of cards) c.classList.toggle("on", c === card);
		};
		return card;
	};
	const cards = [visCard("public", "globe", "Public", "Anyone can read it. You choose who can push."), visCard("private", "lock", "Private", "Only you and the people you give access can read it.")];
	const err = h("div.error-text");
	const check = () => {
		const v = name.value.trim();
		err.textContent = v && !valid(v) ? "Use letters, digits, dots, dashes and underscores (at most 100), not ending in .tit." : "";
		create.disabled = !v || Boolean(err.textContent);
	};
	name.oninput = check;
	const create = btn("Create repository", { primary: true, icon: "plus", type: "submit", disabled: true });
	const form = h(
		"form.card",
		{},
		h(
			"div.card-body",
			{},
			h("div.row", { style: { "align-items": "flex-end", gap: "10px" } }, field("Owner", owner), h("span", { style: { "padding-bottom": "8px", "font-size": "20px", color: "var(--fg-faint)" } }, "/"), h("div.grow", {}, field("Name", name))),
			err,
			h("div.mt-4", {}, field("Description", desc)),
			h("div.mt-4", {}, h("label.label", {}, "Visibility"), h("div.col", { style: { gap: "8px", "margin-top": "6px" } }, cards)),
		),
		h("div.card-foot.row", {}, create, h("span.small.muted", {}, "Then push to it with ", h("code", {}, "tit push"), ".")),
	);
	form.onsubmit = (e) => {
		e.preventDefault();
		busy(create, async () => {
			const body = { name: name.value.trim(), description: desc.value.trim(), visibility: vis };
			if (owner.value !== me.name) body.owner = owner.value;
			const r = await api.post("/repos", body);
			navigate(`/${r.owner || owner.value}/${r.name || body.name}`);
		}).catch(() => {});
	};
	ctx.main.append(h("div.container.slim.page", {}, h("div.page-head", {}, h("div", {}, h("h1", {}, "New repository"), h("p.sub", {}, "A home for a project's history, reviews, benchmarks and releases."))), form, h("p.small.muted.mt-4", {}, "Want a new org instead? ", h("a", { href: "/new/org" }, "Create an org"), ".")));
	setTimeout(() => name.focus(), 30);
}
