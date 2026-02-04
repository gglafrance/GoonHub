<script setup lang="ts">
const explorerStore = useExplorerStore();

const buildUrl = (folderPath: string): string => {
    if (!explorerStore.currentStoragePathID) return '/explorer';
    const cleanPath = folderPath.replace(/^\/+/, '');
    if (cleanPath) {
        return `/explorer/${explorerStore.currentStoragePathID}/${cleanPath}`;
    }
    return `/explorer/${explorerStore.currentStoragePathID}`;
};

const parentUrl = computed(() => {
    if (!explorerStore.currentPath) {
        return '/explorer';
    }
    const parts = explorerStore.currentPath.split('/').filter(Boolean);
    parts.pop();
    const parentPath = parts.length > 0 ? '/' + parts.join('/') : '';
    return buildUrl(parentPath);
});
</script>

<template>
    <div class="flex items-center gap-2">
        <!-- Back Button -->
        <NuxtLink
            :to="parentUrl"
            class="border-border bg-panel hover:border-lava/30 hover:text-lava flex h-8 w-8
                items-center justify-center rounded-lg border transition-all"
        >
            <Icon name="heroicons:arrow-left" size="16" />
        </NuxtLink>

        <!-- Breadcrumb Trail -->
        <nav class="flex min-w-0 flex-1 items-center gap-1 text-sm">
            <!-- Storage Path Name (root) -->
            <NuxtLink
                v-if="explorerStore.currentStoragePath"
                :to="buildUrl('')"
                class="max-w-32 truncate font-medium transition-colors"
                :class="
                    explorerStore.breadcrumbs.length === 0
                        ? 'text-white'
                        : 'text-dim hover:text-lava'
                "
            >
                {{ explorerStore.currentStoragePath.name }}
            </NuxtLink>

            <!-- Folder Path Parts -->
            <template v-for="(crumb, index) in explorerStore.breadcrumbs" :key="crumb.path">
                <Icon name="heroicons:chevron-right" size="14" class="text-dim shrink-0" />
                <NuxtLink
                    :to="buildUrl(crumb.path)"
                    class="max-w-32 truncate transition-colors"
                    :class="
                        index === explorerStore.breadcrumbs.length - 1
                            ? 'font-medium text-white'
                            : 'text-dim hover:text-lava'
                    "
                >
                    {{ crumb.name }}
                </NuxtLink>
            </template>
        </nav>
    </div>
</template>
