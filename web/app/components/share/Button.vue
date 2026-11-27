<script setup lang="ts">
import type { ShareSubject } from '~/types/share';

// The Share button: opens the Share dialog (send to friends, link) on what it is given.
// Hidden when the user may neither send messages nor create links for it. Inside a link
// (marker cards), the click does not follow it. The dialog is mounted on demand (grids hold
// one button per card). getTime: the player position, read when the dialog opens.
const props = withDefaults(
    defineProps<{
        subject: ShareSubject;
        // icon: over a card; row: in a marker row; button: labeled; bar: compact labeled (page bars)
        variant?: 'icon' | 'row' | 'button' | 'bar';
        label?: string;
        getTime?: () => number;
    }>(),
    { variant: 'button', label: 'Share', getTime: undefined },
);

const perms = useMyPermissions();
const allowed = computed(
    () =>
        perms.has('messages:use') ||
        (perms.has('scenes:share') &&
            (props.subject.kind === 'scene' || props.subject.kind === 'marker')),
);
const open = ref(false);
const opened = ref<ShareSubject | null>(null);

const show = (event: MouseEvent) => {
    event.preventDefault();
    event.stopPropagation();
    const s = props.subject;
    opened.value = s.kind === 'scene' && props.getTime ? { ...s, time: props.getTime() } : s;
    open.value = true;
};
</script>

<template>
    <template v-if="allowed">
        <button
            v-if="variant === 'icon'"
            type="button"
            class="hover:bg-lava/80 flex h-6 w-6 items-center justify-center rounded bg-black/70
                text-white/90 backdrop-blur-sm transition-colors"
            :title="label"
            :aria-label="label"
            @click="show"
        >
            <Icon name="heroicons:paper-airplane" size="12" />
        </button>
        <button
            v-else-if="variant === 'row'"
            type="button"
            class="text-dim hover:bg-surface-hover flex shrink-0 items-center justify-center rounded
                p-1.5 opacity-0 transition-all group-hover:opacity-100 hover:text-white
                focus-visible:opacity-100"
            :title="label"
            :aria-label="label"
            @click="show"
        >
            <Icon name="heroicons:paper-airplane" size="12" />
        </button>
        <button
            v-else-if="variant === 'bar'"
            type="button"
            class="border-border bg-panel text-dim hover:border-lava/30 hover:text-lava flex h-6
                shrink-0 items-center gap-1.5 rounded-md border px-2 text-[11px] font-medium
                transition-all"
            :title="label"
            @click="show"
        >
            <Icon name="heroicons:share" size="12" />
            {{ label }}
        </button>
        <button
            v-else
            type="button"
            class="border-border bg-panel text-dim hover:border-lava/30 flex items-center gap-1.5
                rounded-lg border px-2.5 py-1.5 text-[11px] font-medium transition-all
                hover:text-white"
            :title="label"
            @click="show"
        >
            <Icon name="heroicons:share" size="13" />
            {{ label }}
        </button>

        <ShareModal v-if="open && opened" :visible="open" :subject="opened" @close="open = false" />
    </template>
</template>
