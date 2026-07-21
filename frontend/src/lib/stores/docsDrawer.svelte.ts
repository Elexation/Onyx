import { overview, groups, matchesFilter } from "$lib/docs/api-sections.js";

let open = $state(false);
let filterQuery = $state("");

const filteredOverview = $derived(overview.filter((s) => matchesFilter(s, filterQuery)));
const filteredGroups = $derived(
	groups
		.map((g) => ({ ...g, sections: g.sections.filter((s) => matchesFilter(s, filterQuery)) }))
		.filter((g) => g.sections.length > 0),
);
const matchedIds = $derived(
	new Set([
		...filteredOverview.map((s) => s.id),
		...filteredGroups.flatMap((g) => g.sections.map((s) => s.id)),
	]),
);

export const docsDrawer = {
	get open() {
		return open;
	},
	set open(v: boolean) {
		open = v;
	},
	get filterQuery() {
		return filterQuery;
	},
	set filterQuery(v: string) {
		filterQuery = v;
	},
	get filteredOverview() {
		return filteredOverview;
	},
	get filteredGroups() {
		return filteredGroups;
	},
	get matchedIds() {
		return matchedIds;
	},
};
