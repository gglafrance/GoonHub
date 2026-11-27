<script setup lang="ts">
// GoonHub logo, link to the homepage (required header item, top bar only).
</script>

<template>
    <NuxtLink to="/" class="group flex items-center gap-2">
        <div
            class="bg-lava/10 group-hover:bg-lava/20 flex h-7 w-7 items-center justify-center
                rounded-md transition-colors"
        >
            <div class="bg-lava h-2 w-2 rounded-full"></div>
        </div>
        <h1 class="text-sm font-bold tracking-tight text-white">
            GOON<span class="text-lava">HUB</span>
        </h1>
    </NuxtLink>
</template>
