// Per-account layout of the two header bars (docs/NAVIGATION.md), as stored by
// GET/PUT /api/v1/settings/navigation.

export type NavBarId = 'top' | 'nav';
export type NavZoneId = 'left' | 'center' | 'right';

/** Ordered entries of each zone of a bar: item ids or folder references ("folder:<id>"). */
export interface NavBarZones {
    left: string[];
    center: string[];
    right: string[];
}

/** A user-created group of items, shown as a dropdown button. */
export interface NavFolder {
    id: string;
    name: string;
    /** Heroicons name ("heroicons:..."), '' for the default folder icon */
    icon: string;
    items: string[];
}

export interface NavLayout {
    version: 1;
    bars: { top: NavBarZones; nav: NavBarZones };
    folders: NavFolder[];
    /** Items the user hid (never the required ones) */
    hidden: string[];
}

export interface NavLayoutView {
    layout: NavLayout;
    /** false: the user has no stored layout (the default one is returned) */
    customized: boolean;
}
