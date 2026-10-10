// The order replay check verdicts are read in (design/tinhub.md §12.1): what needs a look first.
export const ORDER = ["panicked", "failing", "timeout", "diverged", "skipped", "passed"];

// worst is the verdict that decides a done check's outcome: the first of ORDER among counts ({verdict, count}).
export function worst(counts) {
	const seen = new Set((counts || []).filter((c) => c.count > 0).map((c) => c.verdict));
	return ORDER.find((v) => seen.has(v)) || "";
}
