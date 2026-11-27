/**
 * Drag and drop reordering of a list with the HTML5 drag events: bind `handlers(index)` on
 * each row (`v-bind`), `onMove(from, to)` runs on drop. Rows should also offer move up/down
 * buttons for keyboards and touch screens (no HTML5 drag there).
 */
export function useDragReorder(onMove: (from: number, to: number) => void) {
    const dragIndex = ref<number | null>(null);
    const overIndex = ref<number | null>(null);

    const reset = () => {
        dragIndex.value = null;
        overIndex.value = null;
    };

    const handlers = (index: number) => ({
        draggable: true,
        onDragstart: (e: DragEvent) => {
            dragIndex.value = index;
            if (e.dataTransfer) {
                e.dataTransfer.effectAllowed = 'move';
                e.dataTransfer.setData('text/plain', String(index));
            }
        },
        onDragover: (e: DragEvent) => {
            if (dragIndex.value === null) return;
            e.preventDefault();
            overIndex.value = index;
        },
        onDrop: (e: DragEvent) => {
            e.preventDefault();
            const from = dragIndex.value;
            reset();
            if (from !== null && from !== index) onMove(from, index);
        },
        onDragend: reset,
    });

    return { dragIndex, overIndex, handlers };
}
