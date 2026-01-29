<script setup lang="ts">
const explorerStore = useExplorerStore();
</script>

<template>
    <div>
        <!-- Empty State -->
        <div
            v-if="explorerStore.storagePaths.length === 0 && !explorerStore.isLoading"
            class="border-border flex h-64 flex-col items-center justify-center rounded-xl
                border border-dashed text-center"
        >
            <div
                class="bg-panel border-border flex h-10 w-10 items-center justify-center rounded-lg
                    border"
            >
                <Icon name="heroicons:folder" size="20" class="text-dim" />
            </div>
            <p class="text-muted mt-3 text-sm">No storage paths configured</p>
            <p class="text-dim mt-1 text-xs">Configure storage paths in Settings &gt; Storage</p>
        </div>

        <!-- Storage Path Grid -->
        <div
            v-else
            class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3"
        >
            <ExplorerStoragePathCard
                v-for="storagePath in explorerStore.storagePaths"
                :key="storagePath.id"
                :storage-path="storagePath"
            />
        </div>
    </div>
</template>
