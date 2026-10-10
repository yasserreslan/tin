// Building blocks of the settings pages: a side navigation, titled panels, selects, a user picker with suggestions,
// and a form whose save button reports how it went.

import { h, debounce } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as api from "../lib/api.js";
import { btn, busy, toast } from "./kit.js";

// sidenav is the settings menu: items [{key, label, icon, href} | {group}], the active key highlighted.
export function sidenav(items, active) {
	return h(
		"nav.sidenav",
		{},
		items.filter(Boolean).map((it) => (it.group ? h("div.group", {}, it.group) : h("a", { href: it.href, class: it.key === active ? "active" : "" }, icon(it.icon || "dot"), it.label))),
	);
}

// panel is a titled card: a heading, an optional description, its body and an optional footer.
export function panel(title, desc, body, foot) {
	return h("section.card.mb-5", {}, h("div.card-body", {}, title ? h("h3", { style: { "margin-bottom": desc ? "4px" : "16px" } }, title) : null, desc ? h("p.small.muted", { style: { margin: "0 0 16px" } }, desc) : null, body), foot ? h("div.card-foot.row", {}, foot) : null);
}

export function input(value = "", attrs = {}) {
	return h("input.input", { type: "text", value, ...attrs });
}

export function textarea(value = "", attrs = {}) {
	return h("textarea.textarea", { ...attrs, value });
}

// select from options [[value, label]] or [value].
export function select(options, value, attrs = {}) {
	return h(
		"select.select",
		attrs,
		options.map((o) => {
			const [v, l] = Array.isArray(o) ? o : [o, o];
			return h("option", { value: v, selected: v === value }, l);
		}),
	);
}

export function checkbox(label, checked, hint) {
	const box = h("input", { type: "checkbox", checked });
	const el = h("label.check", {}, box, h("span", {}, h("span", {}, label), hint ? h("div.hint", {}, hint) : null));
	el.input = box;
	return el;
}

let lists = 0;
// userInput is a text input that suggests user names as you type.
export function userInput(attrs = {}) {
	const id = "users-" + ++lists;
	const list = h("datalist", { id });
	const el = h("input.input", { type: "text", list: id, autocomplete: "off", spellcheck: "false", placeholder: "user name", ...attrs });
	el.addEventListener(
		"input",
		debounce(async () => {
			const q = el.value.trim();
			if (!q) return;
			try {
				const r = await api.get("/users", { q });
				list.replaceChildren(...(r.users || []).map((u) => h("option", { value: u.name }, u.display || u.name)));
			} catch {
				/* suggestions are optional */
			}
		}, 150),
	);
	const wrap = h("span.grow", { style: { display: "flex" } }, el, list);
	wrap.input = el;
	return wrap;
}

// saveButton runs save() and says "Saved" or the error.
export function saveButton(save, label = "Save") {
	const b = btn(label, { primary: true });
	b.onclick = () =>
		busy(b, async () => {
			await save();
			toast("Saved");
		}).catch(() => {});
	return b;
}
