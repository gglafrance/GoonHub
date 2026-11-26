<script setup lang="ts">
import type { ResolvedNavBar } from '~/utils/navLayout';

// Secondary row of the header (the nav row): by default the user pages, then the "Manage"
// folder. What it holds comes from the user's layout (docs/NAVIGATION.md); the header leaves
// it out when it is empty.
defineProps<{ zones: ResolvedNavBar }>();
</script>

<template>
    <nav class="border-border bg-void/60 border-b backdrop-blur-sm" aria-label="Main">
        <div class="mx-auto max-w-415 px-4 sm:px-5">
            <HeaderLayoutBar bar="nav" :zones="zones" />
        </div>
    </nav>
</template>
