<script setup lang="ts">
const { modifierLabel } = useCommandPalette();

const hints = computed(() => [
    { keys: ['↑', '↓'], label: 'Navigate' },
    { keys: ['Tab'], label: 'Next group' },
    { keys: ['↵'], label: 'Open' },
    { keys: [modifierLabel.value, '↵'], label: 'New tab' },
]);
</script>

<template>
    <div
        class="border-border bg-surface/60 flex items-center justify-between gap-3 border-t px-4
            py-2"
    >
        <div class="flex flex-wrap items-center gap-x-4 gap-y-1">
            <span
                v-for="hint in hints"
                :key="hint.label"
                class="text-dim flex items-center gap-1.5 text-[10px]"
            >
                <span class="flex items-center gap-0.5">
                    <kbd
                        v-for="key in hint.keys"
                        :key="key"
                        class="border-border bg-panel inline-flex min-w-4 items-center
                            justify-center rounded border px-1 py-px font-mono text-[10px]
                            text-white/80"
                    >
                        {{ key }}
                    </kbd>
                </span>
                {{ hint.label }}
            </span>
        </div>
        <span class="text-dim hidden font-mono text-[10px] tracking-wider uppercase sm:inline">
            Goon<span class="text-lava">Hub</span>
        </span>
    </div>
</template>
