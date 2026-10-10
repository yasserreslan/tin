// The JSON API client (design/tinhub.md §4 and §15). Every call sends the session cookie; an error is an ApiError
// with the status and the API's {code, message}.

export class ApiError extends Error {
	constructor(status, code, message) {
		super(message || code || `HTTP ${status}`);
		this.status = status;
		this.code = code || "";
	}
}

const BASE = "/api/v1";

// enc encodes each path segment of a repository-relative path, keeping the slashes.
export function enc(path) {
	return String(path).split("/").map(encodeURIComponent).join("/");
}

function query(params) {
	if (!params) return "";
	const q = new URLSearchParams();
	for (const [k, v] of Object.entries(params)) {
		if (v === undefined || v === null || v === "") continue;
		q.set(k, String(v));
	}
	const s = q.toString();
	return s ? "?" + s : "";
}

let onUnauthorized = null;

// setUnauthorized is called with every 401 answer (the session ended).
export function setUnauthorized(fn) {
	onUnauthorized = fn;
}

// request sends one call. path starts with "/" and is under /api/v1 unless it starts with "/tit/".
export async function request(method, path, { body, params, raw, signal, accept202 } = {}) {
	const url = (path.startsWith("/tit/") || path.startsWith("/-/") ? path : BASE + path) + query(params);
	const init = { method, credentials: "same-origin", headers: { Accept: "application/json" }, signal };
	if (body !== undefined) {
		init.headers["Content-Type"] = "application/json";
		init.body = JSON.stringify(body);
	}
	let res;
	try {
		res = await fetch(url, init);
	} catch (err) {
		if (err && err.name === "AbortError") throw err;
		throw new ApiError(0, "network", "tinhub can't be reached. Check your connection.");
	}
	if (res.status === 401 && onUnauthorized) onUnauthorized();
	if (!res.ok) {
		let code = "";
		let message = "";
		try {
			const j = await res.json();
			code = j.code || "";
			message = j.message || "";
		} catch {}
		if (!message) message = res.status === 404 ? "Not found" : res.status === 403 ? "You don't have access to this." : `The server answered ${res.status}.`;
		throw new ApiError(res.status, code, message);
	}
	if (raw) return res;
	if (res.status === 204) return null;
	if (res.status === 202 && accept202) return { pending: true, ...(await safeJson(res)) };
	return safeJson(res);
}

async function safeJson(res) {
	const t = await res.text();
	if (!t) return null;
	try {
		return JSON.parse(t);
	} catch {
		return t;
	}
}

export const get = (path, params, opts) => request("GET", path, { params, ...opts });
export const post = (path, body, opts) => request("POST", path, { body: body === undefined ? {} : body, ...opts });
export const put = (path, body, opts) => request("PUT", path, { body: body === undefined ? {} : body, ...opts });
export const patch = (path, body, opts) => request("PATCH", path, { body, ...opts });
export const del = (path, opts) => request("DELETE", path, opts);

// text fetches a blob's bytes as text (UTF-8); bytes as a Blob.
export async function blobText(owner, repo, id) {
	const res = await request("GET", `/repos/${owner}/${repo}/blobs/${id}`, { raw: true });
	return res.text();
}

export async function blobBytes(owner, repo, id) {
	const res = await request("GET", `/repos/${owner}/${repo}/blobs/${id}`, { raw: true });
	return res.blob();
}

// R is a repository's API base: R(o, n)("/refs").
export function R(owner, name) {
	return (rest = "") => `/repos/${encodeURIComponent(owner)}/${encodeURIComponent(name)}${rest}`;
}
