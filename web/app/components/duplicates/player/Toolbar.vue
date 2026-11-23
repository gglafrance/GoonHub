<script setup lang="ts">
// Shared controls of the compare player: play/pause every video, jump to the matching
// segments, shared mute, a seek bar moving every video together (offset from the start of
// their segments), paging and the "hide unchecked" filter.
const props = defineProps<{
    playing: boolean;
    muted: boolean;
    offset: number;
    range: { min: number; max: number };
    page: number;
    pageCount: number;
    // first and last video shown (1-based) and how many there are
    from: number;
    to: number;
    total: number;
    editable: boolean;
    hideUnchecked: boolean;
    uncheckedCount: number;
}>();

const emit = defineEmits<{
    play: [];
    pause: [];
    jump: [];
    toggleMute: [];
    seek: [offset: number];
    seekEnd: [];
    page: [page: number];
    'update:hideUnchecked': [value: boolean];
}>();

const { formatDuration } = useFormatter();

const offsetLabel = computed(
    () => `${props.offset < 0 ? '-' : '+'}${formatDuration(Math.abs(Math.round(props.offset)))}`,
);

const buttonClass =
    'border-border text-dim flex items-center gap-1 rounded-md border px-2 py-1 text-[11px] transition-all hover:text-white disabled:opacity-40';
</script>

<template>
    <div class="space-y-2">
        <div class="flex flex-wrap items-center gap-2">
            <button
                :class="buttonClass"
                :title="playing ? 'Pause every video' : 'Play every video'"
                @click="playing ? emit('pause') : emit('play')"
            >
                <Icon :name="playing ? 'heroicons:pause' : 'heroicons:play'" size="12" />
                {{ playing ? 'Pause all' : 'Play all' }}
            </button>
            <button
                :class="[buttonClass, 'hover:border-lava/30 hover:text-lava']"
                title="Seek every video to the start of its matching segment"
                @click="emit('jump')"
            >
                <Icon name="heroicons:arrows-pointing-in" size="12" />
                Jump to match
            </button>
            <button
                :class="buttonClass"
                :title="muted ? 'Unmute (audio from one video at a time)' : 'Mute'"
                @click="emit('toggleMute')"
            >
                <Icon
                    :name="muted ? 'heroicons:speaker-x-mark' : 'heroicons:speaker-wave'"
                    size="12"
                />
                {{ muted ? 'Muted' : 'Sound on' }}
            </button>
            <label
                v-if="editable"
                class="text-dim flex items-center gap-1.5 text-[11px]"
                title="Only show the videos still checked as eligible"
            >
                <input
                    type="checkbox"
                    class="accent-lava h-3 w-3"
                    :checked="hideUnchecked"
                    @change="
                        emit('update:hideUnchecked', ($event.target as HTMLInputElement).checked)
                    "
                />
                Hide unchecked{{ uncheckedCount ? ` (${uncheckedCount})` : '' }}
            </label>
            <div v-if="pageCount > 1" class="ml-auto flex items-center gap-1 text-[11px]">
                <button :class="buttonClass" :disabled="page <= 0" @click="emit('page', page - 1)">
                    <Icon name="heroicons:chevron-left" size="12" />
                </button>
                <span class="text-dim px-1">
                    {{ from }}-{{ to }} of {{ total }} · page {{ page + 1 }}/{{ pageCount }}
                </span>
                <button
                    :class="buttonClass"
                    :disabled="page >= pageCount - 1"
                    @click="emit('page', page + 1)"
                >
                    <Icon name="heroicons:chevron-right" size="12" />
                </button>
            </div>
        </div>
        <div class="flex items-center gap-2">
            <input
                type="range"
                class="accent-lava h-1 flex-1 cursor-pointer"
                :min="range.min"
                :max="range.max"
                step="0.5"
                :value="offset"
                title="Seek every video together, from the start of its matching segment"
                @input="emit('seek', Number(($event.target as HTMLInputElement).value))"
                @change="emit('seekEnd')"
            />
            <span
                class="text-dim w-20 shrink-0 text-right font-mono text-[10px]"
                title="Position from the start of the matching segments"
            >
                {{ offsetLabel }}
            </span>
        </div>
    </div>
</template>
