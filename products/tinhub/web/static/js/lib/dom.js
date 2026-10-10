// The DOM builder every page uses. Text is always set as text nodes, never parsed as HTML, so a name or a
// comment can never become markup.

const SVG = "http://www.w3.org/2000/svg";

function apply(el, attrs) {
	for (const [k, v] of Object.entries(attrs)) {
		if (v === undefined || v === null || v === false) continue;
		if (k === "class") {
			const c = Array.isArray(v) ? v.filter(Boolean).join(" ") : v;
			if (c) el.setAttribute("class", c);
		} else if (k === "style" && typeof v === "object") {
			for (const [p, pv] of Object.entries(v)) {
				if (pv !== undefined && pv !== null) el.style.setProperty(p, String(pv));
			}
		} else if (k === "dataset") {
			for (const [p, pv] of Object.entries(v)) el.dataset[p] = pv;
		} else if (k.startsWith("on") && typeof v === "function") {
			el.addEventListener(k.slice(2).toLowerCase(), v);
		} else if (k === "ref" && typeof v === "function") {
			v(el);
		} else if (k === "value" && "value" in el) {
			el.value = v;
		} else if (k === "checked" || k === "disabled" || k === "selected" || k === "hidden" || k === "open" || k === "required" || k === "autofocus" || k === "readOnly" || k === "multiple") {
			el[k] = Boolean(v);
		} else if (v === true) {
			el.setAttribute(k, "");
		} else {
			el.setAttribute(k, String(v));
		}
	}
}

function append(el, children) {
	for (const c of children) {
		if (c === null || c === undefined || c === false || c === true) continue;
		if (Array.isArray(c)) append(el, c);
		else if (c instanceof Node) el.appendChild(c);
		else el.appendChild(document.createTextNode(String(c)));
	}
}

// h builds an element: h("a.btn.primary", {href}, "text", child, [children]). The tag may carry .classes and #id.
export function h(tag, attrs, ...children) {
	if (attrs === null || attrs === undefined || typeof attrs !== "object" || Array.isArray(attrs) || attrs instanceof Node) {
		if (attrs !== null && attrs !== undefined) children.unshift(attrs);
		attrs = {};
	}
	const parts = tag.split(/(?=[.#])/);
	const el = document.createElement(parts[0] || "div");
	const cls = [];
	for (const p of parts.slice(1)) {
		if (p[0] === ".") cls.push(p.slice(1));
		else if (p[0] === "#") el.id = p.slice(1);
	}
	if (cls.length) el.className = cls.join(" ");
	if (attrs.class) {
		const extra = Array.isArray(attrs.class) ? attrs.class.filter(Boolean).join(" ") : attrs.class;
		attrs = { ...attrs, class: [el.className, extra].filter(Boolean).join(" ") };
	}
	apply(el, attrs);
	append(el, children);
	return el;
}

// s builds an SVG element.
export function s(tag, attrs, ...children) {
	const el = document.createElementNS(SVG, tag);
	for (const [k, v] of Object.entries(attrs || {})) {
		if (v === undefined || v === null || v === false) continue;
		if (k.startsWith("on") && typeof v === "function") el.addEventListener(k.slice(2).toLowerCase(), v);
		else if (k === "class") el.setAttribute("class", Array.isArray(v) ? v.filter(Boolean).join(" ") : v);
		else el.setAttribute(k, String(v));
	}
	append(el, children);
	return el;
}

export function text(t) {
	return document.createTextNode(String(t));
}

export function frag(...children) {
	const f = document.createDocumentFragment();
	append(f, children);
	return f;
}

export function clear(el) {
	while (el.firstChild) el.removeChild(el.firstChild);
	return el;
}

export function replace(el, ...children) {
	clear(el);
	append(el, children);
	return el;
}

export function $(sel, root = document) {
	return root.querySelector(sel);
}

export function $$(sel, root = document) {
	return Array.from(root.querySelectorAll(sel));
}

// on adds a listener and returns the function that removes it.
export function on(target, type, fn, opts) {
	target.addEventListener(type, fn, opts);
	return () => target.removeEventListener(type, fn, opts);
}

// copy puts text on the clipboard, falling back to a hidden textarea where the API is missing.
export async function copy(t) {
	try {
		await navigator.clipboard.writeText(t);
		return true;
	} catch {
		const ta = h("textarea", { style: { position: "fixed", opacity: "0" } }, t);
		document.body.appendChild(ta);
		ta.select();
		let ok = false;
		try {
			ok = document.execCommand("copy");
		} catch {}
		ta.remove();
		return ok;
	}
}

// debounce delays fn until wait ms pass without a call.
export function debounce(fn, wait) {
	let t = 0;
	return (...args) => {
		clearTimeout(t);
		t = setTimeout(() => fn(...args), wait);
	};
}
