import type { PermissionName } from '~/types/permissions';
import type { PaletteItem } from '~/types/command-palette';
import type { Actor } from '~/types/actor';
import type { Studio } from '~/types/studio';

// Pages shown when the palette input is empty (the main navigation)
const CORE_PAGE_IDS = new Set([
    'page:/',
    'page:/search',
    'page:/actors',
    'page:/studios',
    'page:/playlists',
    'page:/markers',
    'page:/explorer',
    'page:/history',
    'page:/duplicates',
    'page:/settings',
]);

/**
 * Static entries of the command palette: pages, settings tabs, global actions and the
 * shortcuts tied to the scene being watched. Entries the user lacks the permission for are left out.
 */
export function useCommandPaletteCommands() {
    const authStore = useAuthStore();
    const settingsStore = useSettingsStore();
    const route = useRoute();
    const router = useRouter();
    const { enabled: safeModeEnabled, toggle: toggleSafeMode } = useSafeMode();
    const { enabled: hoverAudioEnabled, toggle: toggleHoverAudio } = useHoverAudio();
    const { shortcutsVisible } = useCommandPalette();
    const { searchScenes } = useApiScenes();
    const { fetchSceneActors } = useApiActors();
    const { fetchSceneStudio } = useApiStudios();

    const perms = useMyPermissions();
    const allowed = (item: { permissions?: readonly PermissionName[] }) =>
        !item.permissions || perms.hasAny(...item.permissions);

    const pages = computed<PaletteItem[]>(() => {
        const items: PaletteItem[] = [
            {
                id: 'page:/',
                group: 'pages',
                label: 'Home',
                icon: 'heroicons:home',
                to: '/',
                keywords: ['homepage', 'start'],
            },
            {
                id: 'page:/search',
                group: 'pages',
                label: 'Search',
                icon: 'heroicons:magnifying-glass',
                to: '/search',
                keywords: ['scenes', 'filters', 'browse'],
            },
            {
                id: 'page:/actors',
                group: 'pages',
                label: 'Actors',
                icon: 'heroicons:user-group',
                to: '/actors',
                keywords: ['performers', 'models'],
            },
            {
                id: 'page:/studios',
                group: 'pages',
                label: 'Studios',
                icon: 'heroicons:building-office-2',
                to: '/studios',
            },
            {
                id: 'page:/playlists',
                group: 'pages',
                label: 'Playlists',
                icon: 'heroicons:queue-list',
                to: '/playlists',
            },
            {
                id: 'page:/markers',
                group: 'pages',
                label: 'Markers',
                icon: 'heroicons:bookmark',
                to: '/markers',
                keywords: ['moments', 'labels'],
            },
            {
                id: 'page:/explorer',
                group: 'pages',
                label: 'Explorer',
                icon: 'heroicons:folder',
                to: '/explorer',
                keywords: ['files', 'folders', 'storage'],
                permissions: ['explorer:access'],
            },
            {
                id: 'page:/history',
                group: 'pages',
                label: 'History',
                icon: 'heroicons:clock',
                to: '/history',
                keywords: ['watched', 'activity'],
            },
            {
                id: 'page:/triage',
                group: 'pages',
                label: 'Triage',
                icon: 'heroicons:bolt',
                to: '/triage',
                keywords: ['curate', 'review', 'organize', 'tag', 'rate'],
            },
            {
                id: 'page:/duplicates',
                group: 'pages',
                label: 'Duplicates',
                icon: 'heroicons:document-duplicate',
                to: '/duplicates',
                permissions: ['duplicates:manage'],
                hint: 'Admin',
            },
            {
                id: 'page:/settings',
                group: 'pages',
                label: 'Settings',
                icon: 'heroicons:cog-6-tooth',
                to: '/settings',
                keywords: ['preferences', 'options'],
            },
        ];

        // Homepage tabs (only worth listing when there are several)
        const tabs = settingsStore.settings?.homepage_config?.tabs ?? [];
        if (tabs.length > 1) {
            for (const tab of tabs) {
                items.push({
                    id: `page:/?tab=${tab.id}`,
                    group: 'pages',
                    label: `Home: ${tab.name}`,
                    sublabel: 'Homepage tab',
                    icon: tab.icon || 'heroicons:home',
                    to: `/?tab=${encodeURIComponent(tab.id)}`,
                    keywords: [tab.name],
                });
            }
        }

        // Settings tabs and sub-tabs
        for (const tab of visibleSettingsTabs(SETTINGS_TABS, perms.has)) {
            items.push({
                id: `page:/settings?tab=${tab.id}`,
                group: 'pages',
                label: `Settings: ${tab.label}`,
                sublabel: 'Settings',
                icon: tab.icon,
                to: `/settings?tab=${tab.id}`,
                keywords: [tab.label],
                permissions: tab.permissions,
                hint: tab.admin ? 'Admin' : undefined,
            });
            for (const sub of visibleSettingsSubTabs(tab, perms.has)) {
                const admin = tab.admin || !!sub.permissions;
                items.push({
                    id: `page:/settings?tab=${tab.id}&subtab=${sub.id}`,
                    group: 'pages',
                    label: `Settings: ${tab.label} › ${sub.label}`,
                    sublabel: 'Settings',
                    icon: tab.icon,
                    to: `/settings?tab=${tab.id}&subtab=${sub.id}`,
                    keywords: [sub.label, `${tab.label} ${sub.label}`],
                    permissions: sub.permissions ?? tab.permissions,
                    hint: admin ? 'Admin' : undefined,
                });
            }
        }

        return items.filter(allowed);
    });

    const corePages = computed(() => pages.value.filter((p) => CORE_PAGE_IDS.has(p.id)));

    const actions = computed<PaletteItem[]>(() => [
        {
            id: 'action:random-scene',
            group: 'actions',
            label: 'Play a random scene',
            icon: 'heroicons:arrows-right-left',
            keywords: ['shuffle', 'surprise', 'lucky'],
            run: async () => {
                const seed = Math.floor(Math.random() * Number.MAX_SAFE_INTEGER);
                const res = await searchScenes({ sort: 'random', seed, limit: 1, page: 1 });
                const scene = res?.data?.[0];
                if (scene) await router.push(`/watch/${scene.id}`);
            },
        },
        {
            id: 'action:safe-mode',
            group: 'actions',
            label: safeModeEnabled.value ? 'Disable safe mode' : 'Enable safe mode',
            icon: safeModeEnabled.value ? 'heroicons:eye' : 'heroicons:eye-slash',
            keywords: ['toggle safe mode', 'blur', 'hide thumbnails'],
            run: toggleSafeMode,
        },
        {
            id: 'action:hover-audio',
            group: 'actions',
            label: hoverAudioEnabled.value ? 'Mute preview sound' : 'Unmute preview sound',
            icon: hoverAudioEnabled.value ? 'heroicons:speaker-x-mark' : 'heroicons:speaker-wave',
            keywords: ['toggle hover audio', 'sound', 'volume', 'mute'],
            run: toggleHoverAudio,
        },
        {
            id: 'action:shortcuts',
            group: 'actions',
            label: 'Show keyboard shortcuts',
            icon: 'heroicons:command-line',
            keywords: ['keys', 'hotkeys', 'help'],
            run: () => {
                shortcutsVisible.value = true;
            },
        },
        {
            id: 'action:logout',
            group: 'actions',
            label: 'Log out',
            icon: 'heroicons:arrow-right-on-rectangle',
            keywords: ['sign out', 'logout'],
            run: () => authStore.logout(),
        },
    ]);

    // Entries tied to the scene being watched, loaded when the palette opens on /watch/:id
    const contextItems = ref<PaletteItem[]>([]);
    let contextSceneId = 0;

    const watchedSceneId = computed(() => {
        if (!route.path.startsWith('/watch/')) return 0;
        const id = Number(route.params.id);
        return Number.isFinite(id) ? id : 0;
    });

    const theaterItem = computed<PaletteItem>(() => ({
        id: 'action:theater-mode',
        group: 'context',
        label: settingsStore.theaterMode ? 'Exit theater mode' : 'Enter theater mode',
        icon: 'heroicons:tv',
        keywords: ['toggle theater mode', 'wide', 'cinema'],
        run: () => settingsStore.toggleTheaterMode(),
    }));

    const loadContext = async () => {
        const sceneId = watchedSceneId.value;
        if (!sceneId) {
            contextItems.value = [];
            contextSceneId = 0;
            return;
        }
        if (sceneId === contextSceneId) return;
        contextSceneId = sceneId;
        contextItems.value = [];

        const [actorsRes, studioRes] = await Promise.allSettled([
            fetchSceneActors(sceneId),
            fetchSceneStudio(sceneId),
        ]);
        if (contextSceneId !== sceneId) return;

        const items: PaletteItem[] = [];
        if (actorsRes.status === 'fulfilled') {
            for (const actor of (actorsRes.value?.data ?? []) as Actor[]) {
                items.push({
                    id: `actor:${actor.uuid}`,
                    group: 'actors',
                    label: `Go to ${actor.name}`,
                    sublabel: 'Actor of this scene',
                    icon: 'heroicons:user',
                    image: actor.image_url || undefined,
                    imageShape: 'portrait',
                    to: `/actors/${actor.uuid}`,
                    keywords: [actor.name, 'actor'],
                });
            }
        }
        if (studioRes.status === 'fulfilled' && studioRes.value?.data) {
            const studio = studioRes.value.data as Studio;
            items.push({
                id: `studio:${studio.uuid}`,
                group: 'studios',
                label: `Go to ${studio.name}`,
                sublabel: 'Studio of this scene',
                icon: 'heroicons:building-office-2',
                image: studio.logo || undefined,
                imageShape: 'square',
                to: `/studios/${studio.uuid}`,
                keywords: [studio.name, 'studio'],
            });
        }
        contextItems.value = items;
    };

    const context = computed<PaletteItem[]>(() =>
        watchedSceneId.value ? [...contextItems.value, theaterItem.value] : [],
    );

    return {
        pages,
        corePages,
        actions,
        context,
        loadContext,
        allowed,
        isSuperuser: perms.isSuperuser,
    };
}
