import type { PaletteItem } from '~/types/command-palette';
import type { SceneListItem } from '~/types/scene';
import type { ActorListItem } from '~/types/actor';
import type { StudioListItem } from '~/types/studio';
import type { PlaylistListItem } from '~/types/playlist';
import type { Tag } from '~/types/tag';
import type { SavedSearch } from '~/types/saved_search';

const DEBOUNCE_MS = 200;
const SCENE_LIMIT = 6;
const ENTITY_LIMIT = 5;
const TAG_LIMIT = 6;

const sceneThumbnail = (scene: SceneListItem): string | undefined => {
    if (!scene.thumbnail_path) return undefined;
    const v = scene.updated_at ? new Date(scene.updated_at).getTime() : '';
    return v ? `/thumbnails/${scene.id}?v=${v}` : `/thumbnails/${scene.id}`;
};

/**
 * Server-side search of the command palette: scenes (Meilisearch), actors, studios and
 * playlists through their list endpoints, debounced; tags and saved searches are small lists
 * loaded once per opening and matched locally.
 */
export function useCommandPaletteSearch(query: Ref<string>) {
    const { searchScenes } = useApiScenes();
    const { fetchActors } = useApiActors();
    const { fetchStudios } = useApiStudios();
    const { fetchPlaylists } = useApiPlaylists();
    const { fetchTags } = useApiTags();
    const { fetchSavedSearches } = useApiSavedSearches();
    const { formatDuration } = useFormatter();

    const scenes = ref<PaletteItem[]>([]);
    const actors = ref<PaletteItem[]>([]);
    const studios = ref<PaletteItem[]>([]);
    const playlists = ref<PaletteItem[]>([]);
    const loading = ref(false);

    const allTags = ref<Tag[]>([]);
    const allSavedSearches = ref<SavedSearch[]>([]);

    let timer: ReturnType<typeof setTimeout> | null = null;
    let requestSeq = 0;

    /** Loads the locally matched lists (called when the palette opens). */
    const loadLists = async () => {
        const [tagsRes, savedRes] = await Promise.allSettled([fetchTags(), fetchSavedSearches()]);
        allTags.value = tagsRes.status === 'fulfilled' ? (tagsRes.value?.data ?? []) : [];
        allSavedSearches.value =
            savedRes.status === 'fulfilled' ? (savedRes.value?.data ?? []) : [];
    };

    const clearRemote = () => {
        scenes.value = [];
        actors.value = [];
        studios.value = [];
        playlists.value = [];
    };

    const runSearch = async (q: string) => {
        const seq = ++requestSeq;
        loading.value = true;
        const [scenesRes, actorsRes, studiosRes, playlistsRes] = await Promise.allSettled([
            searchScenes({ q, page: 1, limit: SCENE_LIMIT }),
            fetchActors(1, ENTITY_LIMIT, q),
            fetchStudios(1, ENTITY_LIMIT, q),
            fetchPlaylists({ search: q, page: 1, limit: ENTITY_LIMIT }),
        ]);
        // A newer query was typed meanwhile
        if (seq !== requestSeq) return;

        scenes.value =
            scenesRes.status === 'fulfilled'
                ? ((scenesRes.value?.data ?? []) as SceneListItem[]).map(
                      (scene): PaletteItem => ({
                          id: `scene:${scene.id}`,
                          group: 'scenes',
                          label: scene.title || `Scene #${scene.id}`,
                          sublabel: [
                              scene.duration ? formatDuration(scene.duration) : '',
                              scene.studio ?? '',
                              (scene.actors ?? []).slice(0, 3).join(', '),
                          ]
                              .filter(Boolean)
                              .join(' · '),
                          icon: 'heroicons:film',
                          image: sceneThumbnail(scene),
                          imageShape: 'wide',
                          to: `/watch/${scene.id}`,
                      }),
                  )
                : [];

        actors.value =
            actorsRes.status === 'fulfilled'
                ? ((actorsRes.value?.data ?? []) as ActorListItem[]).map(
                      (actor): PaletteItem => ({
                          id: `actor:${actor.uuid}`,
                          group: 'actors',
                          label: actor.name,
                          sublabel: `${actor.scene_count} scene${actor.scene_count === 1 ? '' : 's'}`,
                          icon: 'heroicons:user',
                          image: actor.image_url || undefined,
                          imageShape: 'portrait',
                          to: `/actors/${actor.uuid}`,
                      }),
                  )
                : [];

        studios.value =
            studiosRes.status === 'fulfilled'
                ? ((studiosRes.value?.data ?? []) as StudioListItem[]).map(
                      (studio): PaletteItem => ({
                          id: `studio:${studio.uuid}`,
                          group: 'studios',
                          label: studio.name,
                          sublabel: `${studio.scene_count} scene${studio.scene_count === 1 ? '' : 's'}`,
                          icon: 'heroicons:building-office-2',
                          image: studio.logo || undefined,
                          imageShape: 'square',
                          to: `/studios/${studio.uuid}`,
                      }),
                  )
                : [];

        playlists.value =
            playlistsRes.status === 'fulfilled'
                ? ((playlistsRes.value?.data ?? []) as PlaylistListItem[]).map(
                      (playlist): PaletteItem => ({
                          id: `playlist:${playlist.uuid}`,
                          group: 'playlists',
                          label: playlist.name,
                          sublabel: [
                              `${playlist.scene_count} scene${playlist.scene_count === 1 ? '' : 's'}`,
                              playlist.owner?.username ?? '',
                          ]
                              .filter(Boolean)
                              .join(' · '),
                          icon: 'heroicons:queue-list',
                          to: `/playlists/${playlist.uuid}`,
                      }),
                  )
                : [];

        loading.value = false;
    };

    watch(query, (value) => {
        if (timer) clearTimeout(timer);
        const q = value.trim();
        if (!q) {
            requestSeq++;
            loading.value = false;
            clearRemote();
            return;
        }
        loading.value = true;
        timer = setTimeout(() => runSearch(q), DEBOUNCE_MS);
    });

    const tags = computed<PaletteItem[]>(() => {
        const q = query.value.trim();
        if (!q) return [];
        return allTags.value
            .map((tag) => ({ tag, score: fuzzyScore(q, tag.name) }))
            .filter((m) => m.score > 0)
            .sort((a, b) => b.score - a.score || b.tag.scene_count - a.tag.scene_count)
            .slice(0, TAG_LIMIT)
            .map(
                ({ tag }): PaletteItem => ({
                    id: `tag:${tag.id}`,
                    group: 'tags',
                    label: tag.name,
                    sublabel: `${tag.scene_count} scene${tag.scene_count === 1 ? '' : 's'}`,
                    icon: 'heroicons:tag',
                    to: `/search?tags=${encodeURIComponent(tag.name)}`,
                }),
            );
    });

    const savedSearches = computed<PaletteItem[]>(() => {
        const q = query.value.trim();
        if (!q) return [];
        return allSavedSearches.value
            .map((saved) => ({ saved, score: fuzzyScore(q, saved.name) }))
            .filter((m) => m.score > 0)
            .sort((a, b) => b.score - a.score)
            .slice(0, ENTITY_LIMIT)
            .map(
                ({ saved }): PaletteItem => ({
                    id: `saved:${saved.uuid}`,
                    group: 'saved',
                    label: saved.name,
                    sublabel: saved.filters.query ? `"${saved.filters.query}"` : 'Saved search',
                    icon: 'heroicons:bookmark-square',
                    to: `/search?saved=${saved.uuid}`,
                }),
            );
    });

    onUnmounted(() => {
        if (timer) clearTimeout(timer);
    });

    return { scenes, actors, studios, tags, playlists, savedSearches, loading, loadLists };
}
