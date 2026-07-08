let isMobile = $state(false);

if (typeof window !== "undefined") {
	const mq = window.matchMedia("(max-width: 767.98px)");
	isMobile = mq.matches;
	mq.addEventListener("change", (e) => {
		isMobile = e.matches;
	});
}

export const viewport = {
	get isMobile() { return isMobile; },
};
