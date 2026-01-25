<script setup lang="ts">
const settingsStore = useSettingsStore();
const authStore = useAuthStore();

useHead({ title: 'Settings' });

type TabType = 'account' | 'player' | 'app' | 'tags' | 'users' | 'jobs';
const activeTab = ref<TabType>('account');

const availableTabs = computed(() => {
    const tabs: TabType[] = ['account', 'player', 'app', 'tags'];
    if (authStore.user?.role === 'admin') {
        tabs.push('users');
        tabs.push('jobs');
    }
    return tabs;
});

onMounted(() => {
    settingsStore.loadSettings();
});

definePageMeta({
    middleware: ['auth'],
});
</script>

<template>
    <div class="mx-auto max-w-2xl px-4 py-8 sm:px-5">
        <h1 class="mb-6 text-lg font-bold tracking-tight text-white">Settings</h1>

        <!-- Tabs -->
        <div class="border-border mb-6 flex gap-1 border-b pb-px">
            <button
                v-for="tab in availableTabs"
                :key="tab"
                @click="activeTab = tab"
                class="relative px-4 py-2 text-xs font-medium capitalize transition-colors"
                :class="activeTab === tab ? 'text-lava' : 'text-dim hover:text-white'"
            >
                {{ tab }}
                <div
                    v-if="activeTab === tab"
                    class="bg-lava absolute right-0 bottom-0 left-0 h-0.5 rounded-full"
                ></div>
            </button>
        </div>

        <SettingsAccount v-if="activeTab === 'account'" />
        <SettingsPlayer v-if="activeTab === 'player'" />
        <SettingsApp v-if="activeTab === 'app'" />
        <SettingsTags v-if="activeTab === 'tags'" />
        <SettingsUsers v-if="activeTab === 'users'" />
        <SettingsJobs v-if="activeTab === 'jobs'" />
    </div>
</template>
