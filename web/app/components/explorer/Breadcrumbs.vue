<script setup lang="ts">
const router = useRouter();
const explorerStore = useExplorerStore();

const buildUrl = (folderPath: string): string => {
    if (!explorerStore.currentStoragePathID) return '/explorer';
    const cleanPath = folderPath.replace(/^\/+/, '');
    if (cleanPath) {
        return `/explorer/${explorerStore.currentStoragePathID}/${cleanPath}`;
    }
    return `/explorer/${explorerStore.currentStoragePathID}`;
};

const navigateUp = () => {
    if (!explorerStore.currentPath) {
        // At root of storage path, go back to storage path list
        router.push('/explorer');
        return;
    }
    // Go to parent folder
    const parts = explorerStore.currentPath.split('/').filter(Boolean);
    parts.pop();
    const parentPath = parts.length > 0 ? '/' + parts.join('/') : '';
    router.push(buildUrl(parentPath));
};

const navigateTo = (folderPath: string) => {
    router.push(buildUrl(folderPath));
};
</script>

<template>
    <div class="flex items-center gap-2">
        <!-- Back Button -->
        <button
            class="border-border bg-panel hover:border-lava/30 hover:text-lava flex h-8 w-8
                items-center justify-center rounded-lg border transition-all"
            @click="navigateUp()"
        >
            <Icon name="heroicons:arrow-left" size="16" />
        </button>

        <!-- Breadcrumb Trail -->
        <nav class="flex min-w-0 flex-1 items-center gap-1 text-sm">
            <!-- Storage Path Name (root) -->
            <button
                v-if="explorerStore.currentStoragePath"
                class="max-w-32 truncate font-medium transition-colors"
                :class="
                    explorerStore.breadcrumbs.length === 0
                        ? 'text-white'
                        : 'text-dim hover:text-lava'
                "
                @click="navigateTo('')"
            >
                {{ explorerStore.currentStoragePath.name }}
            </button>

            <!-- Folder Path Parts -->
            <template v-for="(crumb, index) in explorerStore.breadcrumbs" :key="crumb.path">
                <Icon name="heroicons:chevron-right" size="14" class="text-dim shrink-0" />
                <button
                    class="max-w-32 truncate transition-colors"
                    :class="
                        index === explorerStore.breadcrumbs.length - 1
                            ? 'font-medium text-white'
                            : 'text-dim hover:text-lava'
                    "
                    @click="navigateTo(crumb.path)"
                >
                    {{ crumb.name }}
                </button>
            </template>
        </nav>
    </div>
</template>
