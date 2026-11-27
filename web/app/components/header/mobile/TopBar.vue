<script setup lang="ts">
import type { NavItemDef } from '~/utils/navRegistry';

// Mobile top bar (docs/NAVIGATION.md, "Mobile"): the logo, then the quick icons the user's
// layout shows (search and commands, job status, messages). Everything else is in the bottom
// tab bar and the Menu sheet.
defineProps<{ items: NavItemDef[] }>();

const { open: openPalette } = useCommandPalette();
</script>

<template>
    <header
        class="border-border flex h-12 items-center gap-2 border-b
            pr-[max(0.75rem,env(safe-area-inset-right))] pl-[max(1rem,env(safe-area-inset-left))]"
    >
        <HeaderLogo />
        <div class="ml-auto flex items-center gap-1.5">
            <template v-for="item in items" :key="item.id">
                <button
                    v-if="item.id === 'command_palette'"
                    type="button"
                    class="border-border bg-panel/60 text-dim active:text-lava flex h-9 w-9
                        items-center justify-center rounded-lg border transition-colors"
                    aria-label="Search and commands"
                    @click="openPalette()"
                >
                    <Icon name="heroicons:magnifying-glass" size="18" />
                </button>
                <HeaderJobStatus v-else-if="item.id === 'job_status'" touch />
                <HeaderMessages v-else-if="item.id === 'messages'" touch />
            </template>
        </div>
    </header>
</template>
