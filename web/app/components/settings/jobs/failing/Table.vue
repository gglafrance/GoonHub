<script setup lang="ts">
import type { FailingScene } from '~/types/jobs';
import {
    errorPreview,
    failingPhaseLabel,
    isErrorTruncated,
    toggleId,
    togglePage,
} from '~/utils/failingScenes';

const props = defineProps<{
    items: FailingScene[];
    actionLoading: string | null;
}>();

const emit = defineEmits<{
    retry: [id: number];
    clear: [id: number];
}>();

const selected = defineModel<number[]>('selected', { required: true });

const { formatSize } = useFormatter();

// Rows whose full error is shown
const expanded = ref<number[]>([]);

const pageIds = computed(() => props.items.map((i) => i.id));
const pageSelected = computed(
    () => pageIds.value.length > 0 && pageIds.value.every((id) => selected.value.includes(id)),
);
</script>

<template>
    <div class="overflow-x-auto">
        <table class="w-full text-left text-xs">
            <thead>
                <tr class="text-dim border-border border-b text-[11px] tracking-wider uppercase">
                    <th class="w-6 pr-2 pb-2">
                        <input
                            type="checkbox"
                            class="accent-lava"
                            :checked="pageSelected"
                            title="Select the page"
                            @change="selected = togglePage(selected, pageIds)"
                        />
                    </th>
                    <th class="pr-4 pb-2 font-medium">Scene</th>
                    <th class="pr-4 pb-2 font-medium">Phase</th>
                    <th class="pr-4 pb-2 font-medium">Failures</th>
                    <th class="pr-4 pb-2 font-medium">Last error</th>
                    <th class="pr-4 pb-2 font-medium">Last failure</th>
                    <th class="pr-4 pb-2 font-medium">Size</th>
                    <th class="pb-2 font-medium">Actions</th>
                </tr>
            </thead>
            <tbody>
                <tr
                    v-for="item in props.items"
                    :key="item.id"
                    class="border-border/50 border-b align-top last:border-0 hover:bg-white/2"
                >
                    <td class="py-2.5 pr-2">
                        <input
                            type="checkbox"
                            class="accent-lava"
                            :checked="selected.includes(item.id)"
                            @change="selected = toggleId(selected, item.id)"
                        />
                    </td>
                    <td class="max-w-56 py-2.5 pr-4">
                        <NuxtLink
                            :to="`/watch/${item.scene_id}`"
                            class="hover:text-lava block truncate text-white transition-colors"
                            :title="item.scene_title"
                        >
                            {{ item.scene_title || `Scene #${item.scene_id}` }}
                        </NuxtLink>
                        <div
                            class="text-dim truncate font-mono text-[10px]"
                            :title="item.stored_path"
                        >
                            {{ item.stored_path }}
                        </div>
                    </td>
                    <td class="text-dim py-2.5 pr-4 whitespace-nowrap">
                        {{ failingPhaseLabel(item.phase) }}
                    </td>
                    <td class="py-2.5 pr-4">
                        <span class="text-lava font-medium">{{ item.failures }}</span>
                    </td>
                    <td class="max-w-72 py-2.5 pr-4">
                        <code
                            v-if="expanded.includes(item.id)"
                            class="text-lava block text-[11px] break-all whitespace-pre-wrap"
                        >
                            {{ item.last_error }}
                        </code>
                        <code
                            v-else
                            class="text-lava/90 block text-[11px] break-all"
                            :title="item.last_error"
                        >
                            {{ errorPreview(item.last_error) || 'No error details' }}
                        </code>
                        <button
                            v-if="isErrorTruncated(item.last_error)"
                            class="text-dim mt-0.5 text-[10px] transition-colors hover:text-white"
                            @click="expanded = toggleId(expanded, item.id)"
                        >
                            {{ expanded.includes(item.id) ? 'Less' : 'Full error' }}
                        </button>
                    </td>
                    <td class="text-dim py-2.5 pr-4 text-[11px] whitespace-nowrap">
                        <UiFormattedDate :datetime="item.last_failed_at" time />
                    </td>
                    <td class="text-dim py-2.5 pr-4 text-[11px] whitespace-nowrap">
                        {{ item.file_size != null ? formatSize(item.file_size) : '-' }}
                    </td>
                    <td class="py-2.5">
                        <div class="flex items-center gap-1">
                            <button
                                :disabled="props.actionLoading !== null"
                                class="rounded px-2 py-1 text-[10px] font-medium text-emerald-400
                                    transition-colors hover:bg-emerald-500/10 disabled:opacity-50"
                                title="Forget the failures and queue the job"
                                @click="emit('retry', item.id)"
                            >
                                {{ props.actionLoading === `retry-${item.id}` ? '...' : 'Retry' }}
                            </button>
                            <button
                                :disabled="props.actionLoading !== null"
                                class="text-dim rounded px-2 py-1 text-[10px] font-medium
                                    transition-colors hover:bg-white/5 hover:text-white
                                    disabled:opacity-50"
                                title="Forget the failures without queuing anything"
                                @click="emit('clear', item.id)"
                            >
                                {{ props.actionLoading === `clear-${item.id}` ? '...' : 'Clear' }}
                            </button>
                        </div>
                    </td>
                </tr>
            </tbody>
        </table>
    </div>
</template>
