import type { InjectionKey, Ref } from 'vue';
import type { NavBarId, NavLayout, NavZoneId } from '~/types/navLayout';
import type { NavLocation } from '~/utils/navLayout';

/**
 * State of the header layout editor (Settings > Navigation, docs/NAVIGATION.md): the draft
 * layout, the selected entry (actions toolbar) and the drag and drop in progress. Created by
 * SettingsNavigation and injected by its sub-components (useNavLayoutEditorContext).
 */
export interface NavLayoutEditor {
    draft: Ref<NavLayout>;
    selected: Ref<string | null>;
    dragging: Ref<string | null>;
    /** Drop position shown while dragging: a location and an index in it */
    over: Ref<{ key: string; index: number } | null>;
    select: (entry: string | null) => void;
    move: (entry: string, to: NavLocation, index?: number) => void;
    canPlace: (entry: string, to: NavLocation) => boolean;
}

const NAV_LAYOUT_EDITOR_KEY: InjectionKey<NavLayoutEditor> = Symbol('navLayoutEditor');

/** A stable key of a location (also the value of the "Move to" options). */
export function navLocationKey(loc: NavLocation): string {
    if (loc.kind === 'zone') return `zone:${loc.bar}:${loc.zone}`;
    if (loc.kind === 'folder') return `folder:${loc.folderId}`;
    return 'hidden';
}

/** The location of a key made by navLocationKey, null when it is not one. */
export function parseNavLocationKey(key: string): NavLocation | null {
    if (key === 'hidden') return { kind: 'hidden' };
    const [kind, a, b] = key.split(':');
    if (
        kind === 'zone' &&
        (a === 'top' || a === 'nav') &&
        (b === 'left' || b === 'center' || b === 'right')
    ) {
        return { kind: 'zone', bar: a as NavBarId, zone: b as NavZoneId };
    }
    if (kind === 'folder' && a) return { kind: 'folder', folderId: key.slice('folder:'.length) };
    return null;
}

export function provideNavLayoutEditor(draft: Ref<NavLayout>): NavLayoutEditor {
    const selected = ref<string | null>(null);
    const dragging = ref<string | null>(null);
    const over = ref<{ key: string; index: number } | null>(null);

    const editor: NavLayoutEditor = {
        draft,
        selected,
        dragging,
        over,
        select: (entry) => {
            selected.value = entry;
        },
        move: (entry, to, index) => {
            draft.value = moveNavEntry(draft.value, entry, to, index);
        },
        canPlace: (entry, to) => canPlaceNavEntry(draft.value, entry, to),
    };
    provide(NAV_LAYOUT_EDITOR_KEY, editor);
    return editor;
}

export function useNavLayoutEditorContext(): NavLayoutEditor {
    const editor = inject(NAV_LAYOUT_EDITOR_KEY);
    if (!editor) throw new Error('useNavLayoutEditorContext outside of SettingsNavigation');
    return editor;
}
