import type { Ref } from 'vue';

interface SelectionRect {
    left: number;
    top: number;
    width: number;
    height: number;
}

interface DragSelectOptions {
    containerRef: Ref<HTMLElement | null>;
    onDragEnd: (selectedIds: number[], event: PointerEvent) => void;
    threshold?: number;
}

const INTERACTIVE_SELECTOR =
    'a, button, input, select, textarea, [role="button"], [contenteditable], label';

export function useDragSelect(options: DragSelectOptions) {
    const { containerRef, onDragEnd, threshold = 5 } = options;

    const isDragging = ref(false);
    const selectionRect = ref<SelectionRect | null>(null);
    const dragSelectedIds = ref<Set<number>>(new Set());

    // Start point in page (document) coordinates — stays fixed as user scrolls
    let startPageX = 0;
    let startPageY = 0;
    // Last known pointer position in viewport coordinates
    let lastClientX = 0;
    let lastClientY = 0;
    let rafId = 0;
    let tracking = false;

    function rectsIntersect(a: SelectionRect, b: DOMRect): boolean {
        return !(
            a.left > b.right ||
            a.left + a.width < b.left ||
            a.top > b.bottom ||
            a.top + a.height < b.top
        );
    }

    function computeIntersecting(rect: SelectionRect) {
        const container = containerRef.value;
        if (!container) return;

        const cards = container.querySelectorAll<HTMLElement>('[data-scene-id]');
        const ids = new Set<number>();

        for (const card of cards) {
            const cardRect = card.getBoundingClientRect();
            if (rectsIntersect(rect, cardRect)) {
                const id = Number(card.dataset.sceneId);
                if (!Number.isNaN(id)) ids.add(id);
            }
        }

        dragSelectedIds.value = ids;
    }

    function computeCurrentRect(): SelectionRect {
        // Convert start page coords back to current viewport coords
        const startViewportX = startPageX - window.scrollX;
        const startViewportY = startPageY - window.scrollY;

        return {
            left: Math.min(startViewportX, lastClientX),
            top: Math.min(startViewportY, lastClientY),
            width: Math.abs(lastClientX - startViewportX),
            height: Math.abs(lastClientY - startViewportY),
        };
    }

    function updateSelection() {
        const rect = computeCurrentRect();
        selectionRect.value = rect;
        computeIntersecting(rect);
    }

    function checkThreshold(): boolean {
        const startViewportX = startPageX - window.scrollX;
        const startViewportY = startPageY - window.scrollY;
        return (
            Math.abs(lastClientX - startViewportX) >= threshold ||
            Math.abs(lastClientY - startViewportY) >= threshold
        );
    }

    function scheduleUpdate() {
        cancelAnimationFrame(rafId);
        rafId = requestAnimationFrame(updateSelection);
    }

    function onPointerMove(e: PointerEvent) {
        lastClientX = e.clientX;
        lastClientY = e.clientY;

        if (!isDragging.value) {
            if (!checkThreshold()) return;
            isDragging.value = true;
        }

        scheduleUpdate();
    }

    function onScroll() {
        if (!tracking) return;

        if (!isDragging.value) {
            if (!checkThreshold()) return;
            isDragging.value = true;
        }

        scheduleUpdate();
    }

    function preventDragStart(e: Event) {
        e.preventDefault();
    }

    function cleanupDrag() {
        tracking = false;
        document.removeEventListener('pointermove', onPointerMove);
        document.removeEventListener('pointerup', onPointerUp);
        document.removeEventListener('pointercancel', onPointerCancel);
        document.removeEventListener('scroll', onScroll, true);
        cancelAnimationFrame(rafId);
    }

    function reset() {
        isDragging.value = false;
        selectionRect.value = null;
        dragSelectedIds.value = new Set();
    }

    function onPointerUp(e: PointerEvent) {
        cleanupDrag();
        if (isDragging.value) {
            const ids = [...dragSelectedIds.value];
            reset();
            onDragEnd(ids, e);
        } else {
            reset();
        }
    }

    function onPointerCancel() {
        cleanupDrag();
        reset();
    }

    function onPointerDown(e: PointerEvent) {
        if (e.button !== 0) return;
        if (e.pointerType === 'touch') return;

        const target = e.target as HTMLElement;
        if (!containerRef.value?.contains(target)) return;
        if (!target.closest('[data-scene-id]') && target.closest(INTERACTIVE_SELECTOR)) return;

        startPageX = e.clientX + window.scrollX;
        startPageY = e.clientY + window.scrollY;
        lastClientX = e.clientX;
        lastClientY = e.clientY;
        tracking = true;

        document.addEventListener('pointermove', onPointerMove);
        document.addEventListener('pointerup', onPointerUp);
        document.addEventListener('pointercancel', onPointerCancel);
        document.addEventListener('scroll', onScroll, true);
    }

    onMounted(() => {
        document.addEventListener('pointerdown', onPointerDown, true);
        document.addEventListener('dragstart', preventDragStart, true);
    });

    onUnmounted(() => {
        document.removeEventListener('pointerdown', onPointerDown, true);
        document.removeEventListener('dragstart', preventDragStart, true);
        cleanupDrag();
        reset();
    });

    return {
        isDragging,
        selectionRect,
        dragSelectedIds,
    };
}
