<script setup lang="ts">
const homepageStore = useHomepageStore();

useHead({ title: 'Library' });

useSeoMeta({
    title: 'Library',
    ogTitle: 'Library - GoonHub',
    description: 'Browse your personal video library',
    ogDescription: 'Browse your personal video library',
});

onMounted(async () => {
    await homepageStore.loadHomepage();
});

const handleRefreshSection = async (sectionId: string) => {
    await homepageStore.refreshSection(sectionId);
};

definePageMeta({
    middleware: ['auth'],
});
</script>

<template>
    <div class="min-h-screen px-4 py-6 sm:px-5">
        <div class="mx-auto max-w-415">
            <!-- Upload Section -->
            <VideoUpload v-if="homepageStore.config?.show_upload !== false" />

            <!-- Loading State -->
            <div
                v-if="homepageStore.isLoading && !homepageStore.config"
                class="mt-8 flex h-64 items-center justify-center"
            >
                <LoadingSpinner label="Loading homepage..." />
            </div>

            <!-- Error State -->
            <div
                v-else-if="homepageStore.error"
                class="border-lava/20 bg-lava/5 text-lava mt-8 rounded-lg border px-4 py-3 text-sm"
            >
                {{ homepageStore.error }}
            </div>

            <!-- Dynamic Sections -->
            <div v-else class="mt-8">
                <HomepageSection
                    v-for="section in homepageStore.enabledSections"
                    :key="section.id"
                    :section="section"
                    :data="homepageStore.getSectionData(section.id)"
                    @refresh="handleRefreshSection"
                />

                <!-- Empty State -->
                <div
                    v-if="homepageStore.enabledSections.length === 0 && homepageStore.config"
                    class="border-border flex h-64 flex-col items-center justify-center rounded-xl
                        border border-dashed text-center"
                >
                    <div
                        class="bg-panel border-border flex h-10 w-10 items-center justify-center
                            rounded-lg border"
                    >
                        <Icon name="heroicons:squares-2x2" size="20" class="text-dim" />
                    </div>
                    <p class="text-muted mt-3 text-sm">No sections configured</p>
                    <p class="text-dim mt-1 text-xs">
                        Go to
                        <NuxtLink to="/settings?tab=homepage" class="text-lava hover:underline">
                            Settings
                        </NuxtLink>
                        to customize your homepage
                    </p>
                </div>
            </div>
        </div>
    </div>
</template>
