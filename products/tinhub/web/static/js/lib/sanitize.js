// Raw HTML in Markdown (a README's <p align="center"><img …></p>) made safe: it is parsed by DOMParser into an inert
// document (no scripts run, nothing loads) and copied into fresh elements through an allowlist of tags and
// attributes. Anything else is unwrapped (its text kept) or, for active content, dropped with its children.

import { safeUrl } from "./markdown.js";

const KEEP = new Set("p div span a img br hr b strong i em u s del ins sub sup small kbd code pre blockquote ul ol li h1 h2 h3 h4 h5 h6 table thead tbody tfoot tr th td details summary dl dt dd mark abbr q cite figure figcaption picture caption".split(" "));
const DROP = new Set("script style iframe frame frameset object embed form input textarea select button option noscript template svg math link meta base title head audio video canvas dialog".split(" "));
const ATTRS = {
	a: ["href", "title", "name", "id"],
	img: ["src", "alt", "width", "height", "title"],
	td: ["colspan", "rowspan", "align"],
	th: ["colspan", "rowspan", "align"],
	ol: ["start"],
	details: ["open"],
	abbr: ["title"],
};
const ALIGN = new Set(["p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "td", "th", "img", "figure"]);

function copy(node, out, opts, depth) {
	if (depth > 40) return;
	for (const child of node.childNodes) {
		if (child.nodeType === 3) {
			out.appendChild(document.createTextNode(child.nodeValue));
			continue;
		}
		if (child.nodeType !== 1) continue;
		let tag = child.nodeName.toLowerCase();
		if (DROP.has(tag)) continue;
		if (tag === "center") tag = "div";
		if (!KEEP.has(tag) && tag !== "div") {
			copy(child, out, opts, depth + 1);
			continue;
		}
		const el = document.createElement(tag);
		for (const name of ATTRS[tag] || []) {
			const v = child.getAttribute(name);
			if (v === null) continue;
			if (name === "href") {
				const u = safeUrl(v);
				if (!u) continue;
				const href = opts.link ? opts.link(u) : u;
				el.setAttribute("href", href);
				if (/^https?:/i.test(href)) {
					el.setAttribute("rel", "nofollow noopener noreferrer");
					el.setAttribute("target", "_blank");
				}
			} else if (name === "src") {
				const u = safeUrl(v);
				if (!u) continue;
				const src = opts.image ? opts.image(u) : u;
				if (src) el.setAttribute("src", src);
				el.setAttribute("loading", "lazy");
			} else if (name === "width" || name === "height" || name === "colspan" || name === "rowspan" || name === "start") {
				if (/^\d{1,4}%?$/.test(v)) el.setAttribute(name, v);
			} else if (name === "id" || name === "name") {
				el.setAttribute("id", "user-content-" + v.replace(/[^\w-]/g, ""));
			} else if (name !== "align") el.setAttribute(name, v);
		}
		const align = (child.getAttribute("align") || "").toLowerCase();
		if (ALIGN.has(tag) && (align === "center" || align === "right" || align === "left")) {
			if (tag === "img") el.style.setProperty("float", align === "center" ? "none" : align);
			else el.style.setProperty("text-align", align);
		}
		if (child.nodeName.toLowerCase() === "center") el.style.setProperty("text-align", "center");
		copy(child, el, opts, depth + 1);
		out.appendChild(el);
	}
}

// sanitize turns an HTML string into safe nodes (a fragment). opts.link and opts.image rewrite URLs as Markdown does.
export function sanitize(html, opts = {}) {
	const doc = new DOMParser().parseFromString(`<!doctype html><body>${html}</body>`, "text/html");
	const frag = document.createDocumentFragment();
	copy(doc.body, frag, opts, 0);
	return frag;
}
