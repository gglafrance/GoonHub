export type PaletteGroupId =
    | 'recent'
    | 'context'
    | 'scenes'
    | 'actors'
    | 'studios'
    | 'tags'
    | 'playlists'
    | 'saved'
    | 'pages'
    | 'actions';

export type PaletteImageShape = 'wide' | 'portrait' | 'square';

export interface PaletteItem {
    // Unique across groups, e.g. 'scene:12', 'page:/actors', 'action:random-scene'
    id: string;
    group: PaletteGroupId;
    label: string;
    sublabel?: string;
    icon: string;
    image?: string;
    imageShape?: PaletteImageShape;
    // Route to open (navigation entries)
    to?: string;
    // Command to run (action entries); actions are remembered by id and resolved again
    run?: () => void | Promise<void>;
    keywords?: string[];
    admin?: boolean;
    // Short hint shown on the right (e.g. "Admin", "Tab")
    hint?: string;
}

export interface PaletteGroup {
    id: PaletteGroupId;
    label: string;
    items: PaletteItem[];
}

// What is kept in localStorage for the "Recent" group
export interface PaletteRecentEntry {
    id: string;
    group: PaletteGroupId;
    label: string;
    sublabel?: string;
    icon: string;
    image?: string;
    imageShape?: PaletteImageShape;
    to?: string;
    admin?: boolean;
}
