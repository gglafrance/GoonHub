<script setup lang="ts">
const settingsStore = useSettingsStore();
const authStore = useAuthStore();
const route = useRoute();
const router = useRouter();

useHead({ title: 'Settings' });

useSeoMeta({
    title: 'Settings',
    ogTitle: 'Settings - GoonHub',
    description: 'Configure your GoonHub preferences',
    ogDescription: 'Configure your GoonHub preferences',
});

type TabType = 'account' | 'player' | 'app' | 'homepage' | 'tags' | 'users' | 'jobs' | 'storage';
const activeTab = ref<TabType>('account');

const availableTabs = computed(() => {
    const tabs: TabType[] = ['account', 'player', 'app', 'homepage', 'tags'];
    if (authStore.user?.role === 'admin') {
        tabs.push('users');
        tabs.push('jobs');
        tabs.push('storage');
    }
    return tabs;
});

function setTab(tab: TabType) {
    activeTab.value = tab;
    router.replace({ query: { tab } });
}

onMounted(() => {
    settingsStore.loadSettings();

    const tabFromUrl = route.query.tab as string;
    if (tabFromUrl && availableTabs.value.includes(tabFromUrl as TabType)) {
        activeTab.value = tabFromUrl as TabType;
    } else {
        router.replace({ query: { tab: activeTab.value } });
    }
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
                @click="setTab(tab)"
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
        <SettingsHomepage v-if="activeTab === 'homepage'" />
        <SettingsTags v-if="activeTab === 'tags'" />
        <SettingsUsers v-if="activeTab === 'users'" />
        <SettingsJobs v-if="activeTab === 'jobs'" />
        <SettingsStorage v-if="activeTab === 'storage'" />
    </div>
</template>
