/**
 * Select mode of a scene list, besides its "Select" toggle.
 *
 * A card's hover checkbox selects a scene without the toggle: the first selected scene turns
 * select mode on, and select mode turns off again once that selection is empty. Select mode
 * turned on with the toggle stays on until the toggle turns it off.
 *
 * Escape clears the selection and leaves select mode (`escape: false` when the page already
 * binds it), unless the focus is in a field or a modal is open (Escape closes the modal).
 */
export function useCardSelectMode(
    selectMode: Ref<boolean>,
    hasSelection: Ref<boolean> | ComputedRef<boolean>,
    clearSelection: () => void,
    options: { escape?: boolean } = {},
) {
    // True while select mode is on because of a hover checkbox, not the toggle
    let fromCheckbox = false;

    watch(hasSelection, (has) => {
        if (has && !selectMode.value) {
            fromCheckbox = true;
            selectMode.value = true;
        } else if (!has && fromCheckbox) {
            fromCheckbox = false;
            selectMode.value = false;
        }
    });

    watch(selectMode, (on) => {
        if (!on) fromCheckbox = false;
    });

    if (options.escape === false) return;

    const onKeydown = (e: KeyboardEvent) => {
        if (e.key !== 'Escape' || e.defaultPrevented || !selectMode.value) return;
        const target = e.target as HTMLElement | null;
        if (
            target &&
            (target.tagName === 'INPUT' ||
                target.tagName === 'TEXTAREA' ||
                target.tagName === 'SELECT' ||
                target.isContentEditable)
        ) {
            return;
        }
        // Modals are full-screen fixed overlays
        if (document.querySelector('.fixed.inset-0')) return;
        clearSelection();
        selectMode.value = false;
    };

    onMounted(() => window.addEventListener('keydown', onKeydown));
    onUnmounted(() => window.removeEventListener('keydown', onKeydown));
}
