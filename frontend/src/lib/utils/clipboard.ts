export async function copyToClipboard(
	text: string,
	inputEl?: HTMLInputElement,
): Promise<void> {
	if (navigator.clipboard && window.isSecureContext) {
		await navigator.clipboard.writeText(text);
		return;
	}

	if (inputEl) {
		inputEl.focus();
		inputEl.select();
		inputEl.setSelectionRange(0, text.length);
		const ok = document.execCommand("copy");
		if (!ok) throw new Error("execCommand copy failed");
		return;
	}

	const ta = document.createElement("textarea");
	ta.value = text;
	ta.style.position = "fixed";
	ta.style.opacity = "0";
	document.body.appendChild(ta);
	ta.focus();
	ta.select();
	const ok = document.execCommand("copy");
	document.body.removeChild(ta);
	if (!ok) throw new Error("copy failed");
}
