import type { InjectionKey } from 'vue';

const SCENE_CARD_PREVIEW_KEY: InjectionKey<boolean> = Symbol('sceneCardPreview');

/** Marks the scene card subtree as a preview (card template editor): links must not navigate. */
export function provideSceneCardPreview(preview: boolean) {
    provide(SCENE_CARD_PREVIEW_KEY, preview);
}

/**
 * Preview state for scene card parts. blockInPreview is meant for a capture click listener on a
 * container of links (e.g. a teleported popover), so that nothing navigates away from the editor.
 */
export function useSceneCardPreview() {
    const isPreview = inject(SCENE_CARD_PREVIEW_KEY, false);
    const blockInPreview = (event: Event) => {
        if (!isPreview) return;
        event.preventDefault();
        event.stopPropagation();
    };
    return { isPreview, blockInPreview };
}
