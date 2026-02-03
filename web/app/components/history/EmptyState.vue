<template>
    <div
        class="border-border flex h-64 flex-col items-center justify-center rounded-xl border
            border-dashed text-center"
    >
        <div
            class="bg-panel border-border flex h-10 w-10 items-center justify-center rounded-lg
                border"
        >
            <Icon name="heroicons:clock" size="20" class="text-dim" />
        </div>
        <p class="text-muted mt-3 text-sm">No watch history</p>
        <p class="text-dim mt-1 text-xs">Scenes you watch will appear here</p>
        <NuxtLink
            to="/"
            class="border-border bg-surface text-muted hover:border-border-hover mt-4 rounded-lg
                border px-4 py-2 text-xs font-medium transition-all hover:text-white"
        >
            Browse Library
        </NuxtLink>
    </div>
</template>
