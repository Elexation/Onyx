const SIZE_UNITS = ["B", "KB", "MB", "GB", "TB"];

export function formatFileSize(bytes: number): string {
	if (bytes === 0) return "0 B";
	const i = Math.floor(Math.log(bytes) / Math.log(1000));
	const index = Math.min(i, SIZE_UNITS.length - 1);
	const value = bytes / Math.pow(1000, index);
	return index === 0 ? `${bytes} B` : `${value.toFixed(1)} ${SIZE_UNITS[index]}`;
}

const RELATIVE_THRESHOLDS: [number, Intl.RelativeTimeFormatUnit, number][] = [
	[60, "second", 1],
	[3600, "minute", 60],
	[86400, "hour", 3600],
	[604800, "day", 86400],
];

const rtf = new Intl.RelativeTimeFormat("en", { numeric: "auto" });
const dtf = new Intl.DateTimeFormat("en", { month: "short", day: "numeric", year: "numeric" });

export function formatDate(unixTimestamp: number): string {
	const now = Date.now() / 1000;
	const diff = unixTimestamp - now;
	const absDiff = Math.abs(diff);

	for (const [threshold, unit, divisor] of RELATIVE_THRESHOLDS) {
		if (absDiff < threshold) {
			return rtf.format(Math.round(diff / divisor), unit);
		}
	}

	return dtf.format(unixTimestamp * 1000);
}
