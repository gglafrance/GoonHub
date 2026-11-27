// Mobile navigation (docs/NAVIGATION.md, "Mobile"): below the md breakpoint the two header bars
// are replaced by a compact top bar, a bottom tab bar and a Menu sheet. What goes where is
// derived from the user's resolved layout, so the order and the hidden items of
// Settings > Navigation apply on phones too. Tests in web/tests/mobileNav.test.ts.
import type { NavFolder } from '~/types/navLayout';
import type { NavItemDef } from '~/utils/navRegistry';
import type { ResolvedNavLayout } from '~/utils/navLayout';

/** Media query of the mobile navigation (below Tailwind's md breakpoint). */
export const MOBILE_NAV_QUERY = '(max-width: 767.98px)';

/** Pages of the bottom tab bar, between Home and Menu. */
export const MOBILE_TAB_PAGES = 3;

/** Items shown as icons in the mobile top bar, in this order, when the layout shows them. */
export const MOBILE_QUICK_ITEMS: readonly string[] = ['command_palette', 'job_status', 'messages'];

/** Items left out on phones (keyboard only). */
export const MOBILE_EXCLUDED_ITEMS: readonly string[] = ['shortcuts', 'logo'];

const SOCIAL_ITEMS: readonly string[] = ['friends', 'messages', 'presence'];
const TOGGLE_ITEMS: readonly string[] = ['safe_mode', 'preview_sound'];
const ACCOUNT_ITEMS: readonly string[] = ['settings', 'logout'];

export interface MobileNavSection {
    /** 'pages', or the folder reference ("folder:<id>") */
    key: string;
    title: string;
    icon: string;
    items: NavItemDef[];
}

export interface MobileNav {
    /** Icons of the top bar (palette, job status, messages) */
    quick: NavItemDef[];
    /** Pages of the bottom tab bar (after Home) */
    tabs: NavItemDef[];
    /** Page grids of the Menu sheet: the other pages, then one grid per folder */
    sections: MobileNavSection[];
    /** Rows of the Menu sheet */
    social: NavItemDef[];
    toggles: NavItemDef[];
    account: NavItemDef[];
    /** Whether the user shows the profile item (the Menu sheet header links to the profile) */
    profile: boolean;
}

/** A page of the library (the items of the nav row by default, all links). */
export function isMobilePage(item: NavItemDef): boolean {
    return item.defaultBar === 'nav' && !!item.to;
}

interface FlatEntry {
    item: NavItemDef;
    folder: NavFolder | null;
}

/** Every shown item in header order: the nav row, then the top bar; folders in place. */
function flatten(resolved: ResolvedNavLayout): FlatEntry[] {
    const out: FlatEntry[] = [];
    for (const bar of [resolved.nav, resolved.top]) {
        for (const zone of [bar.left, bar.center, bar.right]) {
            for (const entry of zone) {
                if (entry.kind === 'item') out.push({ item: entry.item, folder: null });
                else for (const item of entry.items) out.push({ item, folder: entry.folder });
            }
        }
    }
    return out;
}

/**
 * Builds the mobile navigation from the resolved layout: the first pages placed directly in a
 * bar (then those in folders) become the tabs; the other pages fill the Menu grids (folders
 * keep their name); the account items become rows. Hidden items stay hidden.
 */
export function buildMobileNav(
    resolved: ResolvedNavLayout,
    tabCount: number = MOBILE_TAB_PAGES,
): MobileNav {
    const flat = flatten(resolved).filter((e) => !MOBILE_EXCLUDED_ITEMS.includes(e.item.id));
    const shown = new Set(flat.map((e) => e.item.id));

    const quick = MOBILE_QUICK_ITEMS.filter((id) => shown.has(id))
        .map((id) => flat.find((e) => e.item.id === id)!.item)
        .filter(Boolean);
    const quickIds = new Set(quick.map((i) => i.id));

    const pages = flat.filter((e) => isMobilePage(e.item));
    const tabs = [...pages.filter((e) => !e.folder), ...pages.filter((e) => e.folder)]
        .slice(0, Math.max(0, tabCount))
        .map((e) => e.item);
    const tabIds = new Set(tabs.map((i) => i.id));

    const sections: MobileNavSection[] = [];
    const loose = pages.filter((e) => !e.folder && !tabIds.has(e.item.id)).map((e) => e.item);
    if (loose.length > 0) {
        sections.push({
            key: 'pages',
            title: 'Pages',
            icon: 'heroicons:squares-2x2',
            items: loose,
        });
    }
    for (const e of pages) {
        if (!e.folder || tabIds.has(e.item.id)) continue;
        const key = `folder:${e.folder.id}`;
        let section = sections.find((s) => s.key === key);
        if (!section) {
            section = {
                key,
                title: e.folder.name,
                icon: e.folder.icon || 'heroicons:folder',
                items: [],
            };
            sections.push(section);
        }
        section.items.push(e.item);
    }

    const pick = (ids: readonly string[]) =>
        flat
            .map((e) => e.item)
            .filter((i) => ids.includes(i.id) && !quickIds.has(i.id))
            .sort((a, b) => ids.indexOf(a.id) - ids.indexOf(b.id));

    return {
        quick,
        tabs,
        sections,
        social: pick(SOCIAL_ITEMS),
        toggles: pick(TOGGLE_ITEMS),
        account: pick(ACCOUNT_ITEMS),
        profile: shown.has('profile'),
    };
}

/** Pages reached from the Menu sheet (its tab looks active on them). */
export function mobileMenuPages(nav: MobileNav): string[] {
    const out: string[] = [];
    for (const s of nav.sections) for (const i of s.items) if (i.to) out.push(i.to);
    for (const i of [...nav.social, ...nav.account]) if (i.to) out.push(i.to);
    return out;
}
