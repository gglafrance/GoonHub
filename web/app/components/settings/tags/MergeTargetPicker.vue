<script setup lang="ts">
import type { Tag } from '~/types/tag';

// Target of a tag merge: one of the candidate tags, or any other existing tag found by search.
const props = defineProps<{
    candidates: Tag[];
    others: Tag[];
    disabled?: boolean;
}>();

const targetId = defineModel<number | null>({ required: true });

const query = ref('');

const otherTarget = computed(() =>
    props.candidates.some((t) => t.id === targetId.value)
        ? undefined
        : props.others.find((t) => t.id === targetId.value),
);

const results = computed(() =>
    query.value.trim()
        ? filterTagsByName(props.others, query.value)
              .filter((t) => t.id !== targetId.value)
              .slice(0, 8)
        : [],
);

function chooseOther(tag: Tag) {
    targetId.value = tag.id;
    query.value = '';
}
</script>

<template>
    <div class="space-y-3">
        <div>
            <p class="text-dim mb-1.5 text-[11px] font-medium tracking-wider uppercase">
                Keep this tag
            </p>
            <div class="space-y-1">
                <label
                    v-for="tag in candidates"
                    :key="tag.id"
                    class="border-border flex cursor-pointer items-center gap-2.5 rounded-lg border
                        px-3 py-1.5 transition-colors hover:bg-white/2"
                    :class="targetId === tag.id ? 'border-lava/40 bg-lava/5' : ''"
                >
                    <input
                        v-model="targetId"
                        type="radio"
                        :value="tag.id"
                        :disabled="disabled"
                        class="accent-lava h-3.5 w-3.5"
                    />
                    <span
                        class="h-2.5 w-2.5 rounded-full"
                        :style="{ backgroundColor: tag.color }"
                    />
                    <span class="flex-1 truncate text-xs whitespace-pre text-white">{{
                        tag.name
                    }}</span>
                    <span class="text-dim text-[11px]">{{ plural(tag.scene_count, 'scene') }}</span>
                </label>
                <div
                    v-if="otherTarget"
                    class="border-lava/40 bg-lava/5 flex items-center gap-2.5 rounded-lg border px-3
                        py-1.5"
                >
                    <Icon name="heroicons:arrow-right-circle" size="14" class="text-lava" />
                    <span
                        class="h-2.5 w-2.5 rounded-full"
                        :style="{ backgroundColor: otherTarget.color }"
                    />
                    <span class="flex-1 truncate text-xs whitespace-pre text-white">{{
                        otherTarget.name
                    }}</span>
                    <span class="text-dim text-[11px]">{{
                        plural(otherTarget.scene_count, 'scene')
                    }}</span>
                </div>
            </div>
        </div>

        <div v-if="others.length > 0" class="relative">
            <input
                v-model="query"
                type="text"
                :disabled="disabled"
                placeholder="Or merge into another tag..."
                class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20 w-full
                    rounded-lg border px-3 py-2 text-xs text-white transition-all focus:ring-1
                    focus:outline-none"
            />
            <div
                v-if="results.length > 0"
                class="border-border bg-panel/95 absolute top-full right-0 left-0 z-10 mt-1 max-h-48
                    overflow-y-auto rounded-lg border p-1 shadow-xl backdrop-blur-md"
            >
                <button
                    v-for="tag in results"
                    :key="tag.id"
                    type="button"
                    class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left
                        transition-colors hover:bg-white/5"
                    @click="chooseOther(tag)"
                >
                    <span
                        class="h-2.5 w-2.5 rounded-full"
                        :style="{ backgroundColor: tag.color }"
                    />
                    <span class="flex-1 truncate text-xs whitespace-pre text-white">{{
                        tag.name
                    }}</span>
                    <span class="text-dim text-[11px]">{{ plural(tag.scene_count, 'scene') }}</span>
                </button>
            </div>
        </div>
    </div>
</template>
