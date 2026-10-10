// The comment composer: a textarea with Write and Preview tabs, Ctrl+Enter to submit.

import { h, clear } from "../lib/dom.js";
import { markdown } from "./code.js";
import { btn, busy } from "./kit.js";

// composer({placeholder, submitLabel, onSubmit(text), onCancel, initial, compact}) returns an element; onSubmit may
// return a promise, and the box clears when it resolves.
export function composer({ placeholder = "Leave a comment", submitLabel = "Comment", onSubmit, onCancel, initial = "", compact = false, extra } = {}) {
	const ta = h("textarea.textarea", { placeholder, rows: compact ? 3 : 5 });
	ta.value = initial;
	const preview = h("div.composer-preview.hidden");
	const write = h("button", { type: "button", class: "on" }, "Write");
	const prev = h("button", { type: "button" }, "Preview");
	write.onclick = () => {
		write.classList.add("on");
		prev.classList.remove("on");
		preview.classList.add("hidden");
		ta.classList.remove("hidden");
		ta.focus();
	};
	prev.onclick = () => {
		prev.classList.add("on");
		write.classList.remove("on");
		clear(preview);
		preview.appendChild(ta.value.trim() ? markdown(ta.value) : h("p.muted", {}, "Nothing to preview."));
		preview.classList.remove("hidden");
		ta.classList.add("hidden");
	};
	const submit = btn(submitLabel, { primary: true, sm: compact });
	const sync = () => (submit.disabled = !ta.value.trim());
	ta.addEventListener("input", sync);
	sync();
	const send = () =>
		busy(submit, async () => {
			if (!ta.value.trim()) return;
			await onSubmit(ta.value);
			ta.value = "";
			sync();
			write.click();
		});
	submit.onclick = send;
	ta.addEventListener("keydown", (e) => {
		if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
			e.preventDefault();
			if (!submit.disabled) send();
		}
		if (e.key === "Escape" && onCancel) onCancel();
	});
	const el = h(
		"div.composer",
		{},
		h("div.composer-tabs", {}, write, prev),
		ta,
		preview,
		h("div.composer-foot", {}, h("span.hint.tiny.grow", {}, "Markdown is supported. ", h("kbd", {}, "Ctrl"), " ", h("kbd", {}, "Enter"), " to send."), extra || null, onCancel ? btn("Cancel", { sm: compact, onclick: onCancel }) : null, submit),
	);
	el.focus = () => ta.focus();
	el.textarea = ta;
	return el;
}
