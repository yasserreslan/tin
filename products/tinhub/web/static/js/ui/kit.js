// Shared components: avatars, badges, buttons, toasts, dialogs, menus, empty and loading states, times and copy
// buttons. Each returns an element.

import { h, copy, on, clear } from "../lib/dom.js";
import { icon } from "../lib/icons.js";
import * as fmt from "../lib/format.js";
import { session } from "../lib/store.js";
import { loginHref } from "../lib/router.js";

export { loginHref };

// avatar is a person's or org's initials on a colour of their name.
export function avatar(name, size = "", { square = false, title } = {}) {
	const hue = fmt.hue(name);
	return h(
		"span",
		{
			class: ["avatar", size, square && "square"],
			title: title || name,
			style: { background: `linear-gradient(135deg, hsl(${hue} 62% 52%), hsl(${(hue + 40) % 360} 58% 42%))` },
			"aria-hidden": "true",
		},
		fmt.initials(name),
	);
}

export function badge(textOrNode, color = "", ic = null, extra = {}) {
	return h("span", { class: ["badge", color], ...extra }, ic ? icon(ic) : null, textOrNode);
}

export function visibility(v) {
	if (v === "private") return badge("Private", "amber outline", "lock");
	if (v === "internal") return badge("Internal", "blue outline", "users");
	return badge("Public", "outline", "globe");
}

const STATE_ICON = { open: "review", approved: "checkCircle", changes_requested: "alert", landed: "landed", abandoned: "xCircle" };

export function stateBadge(state, lg = false) {
	return h("span", { class: ["state", state], style: lg ? null : { height: "24px", padding: "0 9px", "font-size": "11.5px" } }, icon(STATE_ICON[state] || "dot", "sm"), fmt.stateLabel(state));
}

// btn is a button: btn("Save", {primary, icon, onclick, href}).
export function btn(label, opts = {}) {
	const cls = ["btn", opts.primary && "primary", opts.danger && "danger", opts.ghost && "ghost", opts.success && "success", opts.sm && "sm", opts.lg && "lg", !label && "icon", opts.class];
	const kids = [opts.icon ? icon(opts.icon, opts.sm ? "sm" : "") : null, label || null, opts.count !== undefined ? h("span.count", {}, String(opts.count)) : null, opts.caret ? icon("chevronDown", "sm") : null];
	if (opts.href) return h("a", { class: cls, href: opts.href, title: opts.title, "aria-label": opts.title || label }, kids);
	return h("button", { class: cls, type: opts.type || "button", onclick: opts.onclick, disabled: opts.disabled, title: opts.title, "aria-label": opts.title || label }, kids);
}

// busy runs fn with the button disabled and a spinner in place of its icon, and toasts a failure.
export async function busy(button, fn) {
	if (button.disabled) return;
	button.disabled = true;
	const first = button.firstChild;
	const spin = h("span.spinner", { style: { width: "13px", height: "13px" } });
	if (first && first.nodeName === "svg") button.replaceChild(spin, first);
	else button.insertBefore(spin, first);
	try {
		return await fn();
	} catch (err) {
		toast(err.message || String(err), "error");
		throw err;
	} finally {
		button.disabled = false;
		if (spin.parentNode) {
			if (first && first.nodeName === "svg") button.replaceChild(first, spin);
			else spin.remove();
		}
	}
}

// ---- toasts ----

let toastBox = null;

export function toast(message, kind = "ok", { action, timeout = 4200 } = {}) {
	if (!toastBox) {
		toastBox = h("div.toasts", { role: "status", "aria-live": "polite" });
		document.body.appendChild(toastBox);
	}
	const ic = kind === "error" ? "xCircle" : kind === "info" ? "info" : "checkCircle";
	const t = h("div", { class: ["toast", kind] }, icon(ic), h("div.grow", {}, message, action ? [" ", action] : null), h("button.close", { onclick: () => t.remove(), "aria-label": "Dismiss" }, icon("x", "sm")));
	toastBox.appendChild(t);
	if (timeout) setTimeout(() => t.remove(), kind === "error" ? timeout * 1.6 : timeout);
	return t;
}

// ---- dialogs ----

// dialog opens a modal: {title, body, actions: [buttons], wide}; returns {close, el}.
export function dialog({ title, body, actions = [], wide = false, onClose }) {
	const prev = document.activeElement;
	const box = h("div", { class: ["dialog", wide && "wide"], role: "dialog", "aria-modal": "true", "aria-label": title }, h("div.dialog-head", {}, h("h3.grow", {}, title), h("button.btn.ghost.icon.sm", { onclick: () => close(), "aria-label": "Close" }, icon("x", "sm"))), h("div.dialog-body", {}, body), actions.length ? h("div.dialog-foot", {}, actions) : null);
	const overlay = h("div.overlay", { onmousedown: (e) => e.target === overlay && close() }, box);
	const off = on(document, "keydown", (e) => {
		if (e.key === "Escape") {
			e.stopPropagation();
			close();
		}
	});
	let closed = false;
	function close(v) {
		if (closed) return;
		closed = true;
		off();
		overlay.remove();
		if (prev && prev.focus) prev.focus();
		if (onClose) onClose(v);
	}
	document.body.appendChild(overlay);
	const f = box.querySelector("[autofocus], input, textarea, select, .btn.primary");
	if (f) setTimeout(() => f.focus(), 20);
	return { close, el: box };
}

// confirm asks a yes/no question; resolves true when confirmed. typed asks the user to type a word first.
export function confirm({ title, message, confirmLabel = "Confirm", danger = false, typed = "" }) {
	return new Promise((resolve) => {
		let input = null;
		const ok = btn(confirmLabel, { primary: !danger, class: danger ? "danger solid" : "", disabled: Boolean(typed) });
		const body = h("div.stack", {}, h("p.soft", {}, message), typed ? h("div.field", {}, h("label.label", {}, "Type ", h("code", {}, typed), " to confirm."), (input = h("input.input.mono", { autofocus: true, oninput: () => (ok.disabled = input.value !== typed) }))) : null);
		const d = dialog({ title, body, actions: [btn("Cancel", { onclick: () => d.close(false) }), ok], onClose: (v) => resolve(Boolean(v)) });
		ok.onclick = () => d.close(true);
		if (input) input.addEventListener("keydown", (e) => e.key === "Enter" && !ok.disabled && d.close(true));
	});
}

// ---- menus ----

let openMenu = null;

// popover shows content under anchor; it closes on outside click or Escape.
export function popover(anchor, content, { align = "left", onClose, width } = {}) {
	if (openMenu) openMenu.close();
	const el = h("div.popover", { style: width ? { width: width + "px", "max-width": width + "px" } : null }, content);
	document.body.appendChild(el);
	const r = anchor.getBoundingClientRect();
	const w = el.offsetWidth;
	let left = align === "right" ? r.right - w : r.left;
	left = Math.max(8, Math.min(left, window.innerWidth - w - 8));
	let top = r.bottom + 6;
	if (top + el.offsetHeight > window.innerHeight - 8 && r.top - el.offsetHeight - 6 > 8) top = r.top - el.offsetHeight - 6;
	el.style.left = left + "px";
	el.style.top = top + "px";
	const offDown = on(document, "mousedown", (e) => {
		if (!el.contains(e.target) && !anchor.contains(e.target)) close();
	});
	const offKey = on(document, "keydown", (e) => {
		if (e.key === "Escape") close();
		if (e.key === "ArrowDown" || e.key === "ArrowUp") {
			const items = [...el.querySelectorAll(".menu-item")];
			if (!items.length) return;
			e.preventDefault();
			const i = items.indexOf(document.activeElement);
			const next = e.key === "ArrowDown" ? (i + 1) % items.length : (i - 1 + items.length) % items.length;
			items[next].focus();
		}
	});
	const offScroll = on(window, "resize", () => close());
	function close() {
		offDown();
		offKey();
		offScroll();
		el.remove();
		if (openMenu && openMenu.el === el) openMenu = null;
		if (onClose) onClose();
	}
	openMenu = { el, close };
	return openMenu;
}

// menu shows items [{label, icon, onclick | href, danger, checked, sep, head}] under anchor.
export function menu(anchor, items, opts = {}) {
	let pop = null;
	const content = items.map((it) => {
		if (it.sep) return h("div.menu-sep");
		if (it.head) return h("div.menu-head", {}, it.head);
		if (it.node) return it.node;
		const kids = [it.icon ? icon(it.icon) : null, h("span.grow.ellipsis", {}, it.label), it.hint ? h("span.faint.tiny", {}, it.hint) : null, it.checked ? h("span.check-mark", {}, icon("check", "sm")) : null];
		const click = (e) => {
			pop.close();
			if (it.onclick) it.onclick(e);
		};
		if (it.href) return h("a", { class: ["menu-item", it.danger && "danger"], href: it.href, onclick: click }, kids);
		return h("button", { class: ["menu-item", it.danger && "danger"], type: "button", onclick: click }, kids);
	});
	pop = popover(anchor, content, opts);
	return pop;
}

// picker is a filterable list in a popover: items [{value, label, sub, icon}], onPick(value).
export function picker(anchor, { items, placeholder = "Filter", onPick, current, width = 300, footer }) {
	const input = h("input.input", { placeholder, autofocus: true });
	const list = h("div");
	const draw = () => {
		clear(list);
		const q = input.value.toLowerCase();
		const shown = items.filter((it) => !q || it.label.toLowerCase().includes(q) || (it.sub || "").toLowerCase().includes(q)).slice(0, 100);
		if (!shown.length) list.appendChild(h("div.menu-head", { style: { "text-transform": "none" } }, "Nothing matches."));
		for (const it of shown) {
			if (it.head) {
				list.appendChild(h("div.menu-head", {}, it.head));
				continue;
			}
			list.appendChild(
				h(
					"button.menu-item",
					{
						type: "button",
						onclick: () => {
							pop.close();
							onPick(it.value);
						},
					},
					it.icon ? icon(it.icon) : null,
					h("span.grow.ellipsis", {}, it.label, it.sub ? h("span.faint", {}, "  " + it.sub) : null),
					it.value === current ? h("span.check-mark", {}, icon("check", "sm")) : null,
				),
			);
		}
	};
	input.addEventListener("input", draw);
	input.addEventListener("keydown", (e) => {
		if (e.key === "Enter") {
			const first = list.querySelector(".menu-item");
			if (first) first.click();
		}
	});
	draw();
	const pop = popover(anchor, [h("div.menu-filter", {}, input), list, footer || null], { width });
	setTimeout(() => input.focus(), 10);
	return pop;
}

// ---- states ----

export function empty(ic, title, message, action) {
	return h("div.empty", {}, h("div.art", {}, icon(ic)), h("h3", {}, title), message ? h("p", {}, message) : null, action || null);
}

export function loading(label = "Loading") {
	return h("div.loading", {}, h("span.spinner"), label + "…");
}

export function skeleton(lines = 4) {
	return h("div", { style: { padding: "8px 16px" } }, Array.from({ length: lines }, (_, i) => h("div.skeleton.skel-line", { style: { width: `${90 - ((i * 17) % 40)}%` } })));
}

// signInFirst is what a page shows a signed-out visitor where the server refused them: the fix is to sign in, not to
// ask for a permission.
export function signInFirst(title = "Sign in to see this", message = "This needs an account with access to it.") {
	return empty("login", title, message, btn("Sign in", { icon: "login", primary: true, href: loginHref() }));
}

// refused is the state for a 403: a sign-in prompt when no one is signed in, else no access with the server's reason.
export function refused(err, title, message, signedOut) {
	if (!session.user) return signedOut || signInFirst();
	return empty("lock", title, message || (err && err.message) || "You don't have access to this.");
}

export function errorBox(err, retry) {
	const status = err && err.status;
	if ((status === 403 || status === 401) && !session.user) return signInFirst();
	// A private repository is a 404 to someone signed out, so the way in is to sign in.
	if (status === 404 && !session.user) return empty("search", "Not found", `${(err && err.message) || "Not found"}. If it is private, sign in to see it.`, btn("Sign in", { icon: "login", primary: true, href: loginHref() }));
	const title = status === 404 ? "Not found" : status === 403 ? "No access" : status === 0 ? "Offline" : "Something went wrong";
	return h("div.empty", {}, h("div.art", {}, icon(status === 404 ? "search" : status === 403 ? "lock" : "alert")), h("h3", {}, title), h("p", {}, err && err.message ? err.message : String(err)), retry && status !== 404 && status !== 403 ? btn("Try again", { icon: "refresh", onclick: retry }) : null);
}

export function callout(kind, ...children) {
	const ic = kind === "warn" ? "alert" : kind === "error" ? "xCircle" : kind === "ok" ? "checkCircle" : "info";
	return h("div", { class: ["callout", kind] }, icon(ic), h("div.grow", {}, children));
}

// ---- time ----

const timeEls = new Set();
setInterval(() => {
	for (const el of timeEls) {
		if (!el.isConnected) timeEls.delete(el);
		else el.textContent = fmt.ago(Number(el.dataset.t));
	}
}, 30000);

// time is a relative time that keeps itself current, with the full date as its title.
export function time(t, { prefix = "" } = {}) {
	const ms = fmt.toMs(t);
	if (!ms) return h("span.faint", {}, "never");
	const el = h("time", { datetime: fmt.iso(ms), title: fmt.date(ms, true), dataset: { t: String(ms) } }, fmt.ago(ms));
	timeEls.add(el);
	return prefix ? h("span", {}, prefix, el) : el;
}

// ---- copy ----

export function copyButton(textOrFn, { label = "", sm = true, title = "Copy" } = {}) {
	const b = h("button", { class: ["btn", sm && "sm", !label && "icon", "ghost"], type: "button", title, "aria-label": title }, icon("copy", "sm"), label || null);
	b.addEventListener("click", async () => {
		const t = typeof textOrFn === "function" ? textOrFn() : textOrFn;
		const ok = await copy(t);
		const ic = b.querySelector("svg");
		if (ok && ic) {
			const done = icon("check", "sm");
			done.style.color = "var(--green)";
			b.replaceChild(done, ic);
			setTimeout(() => done.parentNode && b.replaceChild(ic, done), 1400);
		}
	});
	return b;
}

// ---- links ----

export function userLink(name, { display, avatarSize = "sm", plain = false } = {}) {
	if (!name) return h("span.muted", {}, "someone");
	const who = name.includes("@") ? name.split("@")[0] : name;
	return h("a", { class: ["row", plain && "plain"], href: "/" + encodeURIComponent(who), style: { display: "inline-flex", gap: "6px", "font-weight": "600", color: "var(--fg)" } }, avatarSize ? avatar(display || who, avatarSize) : null, display || who);
}

export function repoLink(full, { icon: ic = true } = {}) {
	const [o, n] = full.split("/");
	return h("a", { href: `/${o}/${n}`, class: "row", style: { display: "inline-flex", gap: "6px" } }, ic ? icon("repo", "sm") : null, h("span", {}, o, h("span.faint", {}, " / "), h("b", {}, n)));
}

// more is a "Load more" button that calls load() until it returns false.
export function more(load) {
	const b = btn("Load more", { icon: "chevronDown" });
	const wrap = h("div.center", { style: { padding: "16px" } }, b);
	b.onclick = () =>
		busy(b, async () => {
			const again = await load();
			if (!again) wrap.remove();
		});
	return wrap;
}

let fieldIds = 0;

// field wraps an input with its label and hint. The label names the field's control (the input itself, or the only
// one inside it, not a group of checkboxes), so a click on it focuses the control and a screen reader reads it.
export function field(label, input, hint, error) {
	let lab = null;
	if (label) {
		lab = h("label.label", {}, label);
		const ctls = input instanceof Element ? (input.matches("input, select, textarea") ? [input] : [...input.querySelectorAll("input, select, textarea")]) : [];
		const ctl = ctls.length === 1 && ctls[0].type !== "checkbox" && ctls[0].type !== "radio" ? ctls[0] : null;
		if (ctl) {
			if (!ctl.id) ctl.id = `field-${++fieldIds}`;
			lab.htmlFor = ctl.id;
		}
	}
	return h("div.field", {}, lab, input, hint ? h("div.hint", {}, hint) : null, error ? h("div.error-text", {}, error) : null);
}

// tabs is a tab bar of links: [{label, href, icon, count, active}].
export function tabs(items, cls = "") {
	return h("nav", { class: ["tabs", cls] }, items.map((t) => h("a", { href: t.href, class: t.active ? "active" : "", "aria-current": t.active ? "page" : null }, t.icon ? icon(t.icon) : null, t.label, t.count ? h("span.counter", {}, String(t.count)) : null)));
}

// segmented is a small switch: options [{value, label, icon}], onChange(value).
export function segmented(options, value, onChange) {
	const el = h("div.segmented", { role: "group" });
	const draw = (v) => {
		clear(el);
		for (const o of options) el.appendChild(h("button", { type: "button", class: o.value === v ? "on" : "", title: o.title || o.label, onclick: () => (draw(o.value), onChange(o.value)) }, o.icon ? icon(o.icon, "sm") : null, o.label ? " " + o.label : null));
	};
	draw(value);
	return el;
}

// kv is a definition list from [[label, value]].
export function kv(rows) {
	return h("dl.kv", {}, rows.filter(Boolean).map(([k, v]) => [h("dt", {}, k), h("dd", {}, v)]));
}

// person shows a commit person: avatar, name and time.
export function person(p, verb = "") {
	if (!p) return null;
	return h("span.row", { style: { gap: "6px", display: "inline-flex" } }, avatar(p.name || p.email || "?", "sm"), h("b", {}, p.name || p.email), verb ? h("span.muted", {}, verb) : null, p.when ? time(p.when) : null);
}
