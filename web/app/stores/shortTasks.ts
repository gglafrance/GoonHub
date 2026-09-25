import type { ShortEventData, ShortTask } from '~/types/shorts';

/**
 * Shorts being encoded, fed by the short:* SSE events. The watch page's Shorts
 * tab shows them as progress cards and reloads its list when one completes.
 */
export const useShortTasksStore = defineStore('shortTasks', () => {
    const tasks = ref<Record<string, ShortTask>>({});
    // Bumped per source scene each time one of its shorts finishes.
    const completedTick = ref<Record<number, number>>({});

    const forSource = (sourceId: number) =>
        Object.values(tasks.value).filter((t) => t.source_scene_id === sourceId);

    const upsert = (task: ShortTask) => {
        tasks.value = { ...tasks.value, [task.task_id]: task };
    };

    const remove = (taskId: string) => {
        tasks.value = Object.fromEntries(
            Object.entries(tasks.value).filter(([id]) => id !== taskId),
        );
    };

    const handleEvent = (eventType: string, data: ShortEventData) => {
        const existing = tasks.value[data.task_id];
        if (eventType === 'short:progress') {
            upsert({
                task_id: data.task_id,
                source_scene_id: data.source_scene_id,
                start: data.start ?? existing?.start ?? 0,
                end: data.end ?? existing?.end ?? 0,
                title: data.title ?? existing?.title ?? '',
                percent: data.percent ?? 0,
                status: 'running',
            });
        } else if (eventType === 'short:completed') {
            remove(data.task_id);
            completedTick.value = {
                ...completedTick.value,
                [data.source_scene_id]: (completedTick.value[data.source_scene_id] ?? 0) + 1,
            };
        } else if (eventType === 'short:failed') {
            upsert({
                task_id: data.task_id,
                source_scene_id: data.source_scene_id,
                start: existing?.start ?? 0,
                end: existing?.end ?? 0,
                title: existing?.title ?? '',
                percent: existing?.percent ?? 0,
                status: 'failed',
                error: data.error,
            });
        }
    };

    return { tasks, completedTick, forSource, upsert, remove, handleEvent };
});
