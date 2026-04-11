export type SortField = "name" | "size" | "modified" | "type";
export type SortDir = "asc" | "desc";
export type ViewMode = "list" | "grid";

interface Preferences {
	viewMode: ViewMode;
	sortField: SortField;
	sortDir: SortDir;
	showHidden: boolean;
	directoryOverrides: Record<string, ViewMode>;
}

const STORAGE_KEY = "onyx-preferences";

const DEFAULTS: Preferences = {
	viewMode: "list",
	sortField: "name",
	sortDir: "asc",
	showHidden: false,
	directoryOverrides: {},
};

function loadPreferences(): Preferences {
	try {
		const raw = localStorage.getItem(STORAGE_KEY);
		if (!raw) return { ...DEFAULTS };
		return { ...DEFAULTS, ...JSON.parse(raw) };
	} catch {
		return { ...DEFAULTS };
	}
}

function save(prefs: Preferences) {
	localStorage.setItem(STORAGE_KEY, JSON.stringify(prefs));
}

let prefs = $state<Preferences>(loadPreferences());

export const preferences = {
	get viewMode() { return prefs.viewMode; },
	set viewMode(v: ViewMode) { prefs.viewMode = v; save(prefs); },

	get sortField() { return prefs.sortField; },
	set sortField(v: SortField) { prefs.sortField = v; save(prefs); },

	get sortDir() { return prefs.sortDir; },
	set sortDir(v: SortDir) { prefs.sortDir = v; save(prefs); },

	get showHidden() { return prefs.showHidden; },
	set showHidden(v: boolean) { prefs.showHidden = v; save(prefs); },

	getDirectoryOverride(dirPath: string): ViewMode | undefined {
		return prefs.directoryOverrides[dirPath];
	},

	setDirectoryViewMode(dirPath: string, mode: ViewMode) {
		prefs.directoryOverrides[dirPath] = mode;
		save(prefs);
	},

	clearDirectoryOverride(dirPath: string) {
		delete prefs.directoryOverrides[dirPath];
		save(prefs);
	},
};
