<script setup lang="ts">
// Compact reminder of the triage keys, under the scene.
defineEmits<{ help: [] }>();
</script>

<template>
    <div class="flex flex-wrap items-center gap-x-3 gap-y-1.5">
        <span
            v-for="shortcut in TRIAGE_SHORTCUTS"
            :key="shortcut.hint"
            class="text-dim flex items-center gap-1 text-[10px]"
        >
            <kbd
                class="border-border bg-panel rounded border px-1 py-px font-mono text-[10px]
                    text-white"
                >{{ shortcut.keys[0] }}</kbd
            >
            {{ shortcut.hint }}
        </span>
        <button
            class="text-dim hover:text-lava ml-auto text-[10px] transition-colors"
            @click="$emit('help')"
        >
            All shortcuts
        </button>
    </div>
</template>
