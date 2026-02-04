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

type TabType =
    | 'account'
    | 'player'
    | 'app'
    | 'homepage'
    | 'tags'
    | 'parsing-rules'
    | 'users'
    | 'jobs'
    | 'storage'
    | 'trash';

type JobsSubTabType =
    | 'manual'
    | 'history'
    | 'workers'
    | 'processing'
    | 'triggers'
    | 'retry'
    | 'dlq';

interface TabConfig {
    id: TabType;
    label: string;
    icon: string;
    admin?: boolean;
    subTabs?: { id: JobsSubTabType; label: string }[];
}

const tabConfig: TabConfig[] = [
    { id: 'account', label: 'Account', icon: 'heroicons:user-circle' },
    { id: 'player', label: 'Player', icon: 'heroicons:play-circle' },
    { id: 'app', label: 'App', icon: 'heroicons:cog-6-tooth' },
    { id: 'homepage', label: 'Homepage', icon: 'heroicons:home' },
    { id: 'tags', label: 'Tags', icon: 'heroicons:tag' },
    { id: 'parsing-rules', label: 'Parsing Rules', icon: 'heroicons:funnel' },
    { id: 'users', label: 'Users', icon: 'heroicons:users', admin: true },
    {
        id: 'jobs',
        label: 'Jobs',
        icon: 'heroicons:queue-list',
        admin: true,
        subTabs: [
            { id: 'manual', label: 'Manual' },
            { id: 'history', label: 'History' },
            { id: 'workers', label: 'Workers' },
            { id: 'processing', label: 'Processing' },
            { id: 'triggers', label: 'Triggers' },
            { id: 'retry', label: 'Retry' },
            { id: 'dlq', label: 'DLQ' },
        ],
    },
    { id: 'storage', label: 'Storage', icon: 'heroicons:folder', admin: true },
    { id: 'trash', label: 'Trash', icon: 'heroicons:trash', admin: true },
];

const activeTab = ref<TabType>('account');
const activeSubTab = ref<JobsSubTabType>('manual');
const expandedTabs = ref<Set<TabType>>(new Set());

const availableTabs = computed(() => {
    return tabConfig.filter((tab) => !tab.admin || authStore.user?.role === 'admin');
});

const regularTabs = computed(() => availableTabs.value.filter((t) => !t.admin));
const adminTabs = computed(() => availableTabs.value.filter((t) => t.admin));

function setTab(tab: TabConfig, subTab?: JobsSubTabType) {
    activeTab.value = tab.id;

    if (tab.subTabs) {
        // If clicking a tab with sub-tabs, toggle expansion
        if (!subTab) {
            if (expandedTabs.value.has(tab.id)) {
                expandedTabs.value.delete(tab.id);
            } else {
                expandedTabs.value.add(tab.id);
            }
        } else {
            // If clicking a sub-tab, ensure parent is expanded
            expandedTabs.value.add(tab.id);
            activeSubTab.value = subTab;
        }
    }

    updateUrl();
}

function setSubTab(tab: TabConfig, subTab: JobsSubTabType) {
    activeTab.value = tab.id;
    activeSubTab.value = subTab;
    expandedTabs.value.add(tab.id);
    updateUrl();
}

function toggleExpand(tab: TabConfig, event: Event) {
    event.stopPropagation();
    if (expandedTabs.value.has(tab.id)) {
        expandedTabs.value.delete(tab.id);
    } else {
        expandedTabs.value.add(tab.id);
    }
}

function updateUrl() {
    const query: Record<string, string> = { tab: activeTab.value };
    const currentTab = tabConfig.find((t) => t.id === activeTab.value);
    if (currentTab?.subTabs) {
        query.subtab = activeSubTab.value;
    }
    router.replace({ query });
}

function isTabActive(tab: TabConfig) {
    return activeTab.value === tab.id;
}

function isSubTabActive(subTabId: JobsSubTabType) {
    return activeSubTab.value === subTabId;
}

onMounted(() => {
    settingsStore.loadSettings();

    const tabFromUrl = route.query.tab as string;
    const subTabFromUrl = route.query.subtab as string;

    if (tabFromUrl) {
        const tab = availableTabs.value.find((t) => t.id === tabFromUrl);
        if (tab) {
            activeTab.value = tab.id;

            if (tab.subTabs && subTabFromUrl) {
                const validSubTab = tab.subTabs.find((st) => st.id === subTabFromUrl);
                if (validSubTab) {
                    activeSubTab.value = validSubTab.id;
                    expandedTabs.value.add(tab.id);
                }
            }
        }
    }

    updateUrl();
});

definePageMeta({
    middleware: ['auth'],
});
</script>

<template>
    <div class="flex h-[calc(100vh-6rem)] flex-col overflow-hidden lg:flex-row">
        <!-- Mobile: Horizontal scrollable tabs -->
        <div class="border-border bg-surface/50 shrink-0 border-b backdrop-blur-sm lg:hidden">
            <!-- Main tabs row -->
            <div class="scrollbar-none -mx-px flex overflow-x-auto px-4 py-2">
                <template v-for="tab in regularTabs" :key="tab.id">
                    <button
                        @click="setTab(tab)"
                        class="flex shrink-0 flex-col items-center gap-1 rounded-lg px-4 py-2
                            transition-all"
                        :class="
                            isTabActive(tab)
                                ? 'bg-lava/10 text-lava'
                                : 'text-dim hover:bg-white/5 hover:text-white'
                        "
                    >
                        <Icon :name="tab.icon" size="20" />
                        <span class="text-[10px] font-medium">{{ tab.label }}</span>
                    </button>
                </template>

                <!-- Admin divider -->
                <div
                    v-if="adminTabs.length > 0"
                    class="mx-2 my-auto h-8 w-px shrink-0 bg-white/10"
                />

                <template v-for="tab in adminTabs" :key="tab.id">
                    <button
                        @click="setTab(tab)"
                        class="flex shrink-0 flex-col items-center gap-1 rounded-lg px-4 py-2
                            transition-all"
                        :class="
                            isTabActive(tab)
                                ? 'bg-lava/10 text-lava'
                                : 'text-dim hover:bg-white/5 hover:text-white'
                        "
                    >
                        <Icon :name="tab.icon" size="20" />
                        <span class="text-[10px] font-medium">{{ tab.label }}</span>
                    </button>
                </template>

                <!-- End spacer for scroll -->
                <div class="w-4 shrink-0" />
            </div>

            <!-- Sub-tabs row (for Jobs) -->
            <div
                v-if="activeTab === 'jobs' && tabConfig.find((t) => t.id === 'jobs')?.subTabs"
                class="border-border scrollbar-none flex gap-1 overflow-x-auto border-t px-4 py-2"
            >
                <button
                    v-for="subTab in tabConfig.find((t) => t.id === 'jobs')?.subTabs"
                    :key="subTab.id"
                    @click="setSubTab(tabConfig.find((t) => t.id === 'jobs')!, subTab.id)"
                    class="shrink-0 rounded-full px-3 py-1 text-xs font-medium transition-all"
                    :class="
                        isSubTabActive(subTab.id)
                            ? 'bg-lava text-white'
                            : 'text-dim bg-white/5 hover:bg-white/10 hover:text-white'
                    "
                >
                    {{ subTab.label }}
                </button>
            </div>
        </div>

        <!-- Desktop: Sidebar (unchanged) -->
        <aside
            class="sticky top-16 hidden h-[calc(100vh-5.25rem)] w-55 shrink-0 border-r
                border-white/8 bg-[rgba(10,10,10,0.5)] backdrop-blur-xl lg:block"
        >
            <div class="p-4">
                <h1 class="mb-5 text-sm font-semibold tracking-tight text-white">Settings</h1>

                <!-- Regular tabs -->
                <nav class="space-y-0.5">
                    <template v-for="tab in regularTabs" :key="tab.id">
                        <button
                            @click="setTab(tab)"
                            class="group flex w-full items-center gap-2.5 rounded-lg px-3 py-2
                                text-[13px] font-medium transition-all duration-150"
                            :class="
                                isTabActive(tab)
                                    ? 'bg-lava/10 text-lava shadow-[inset_3px_0_0_#ff4d4d]'
                                    : 'text-white/50 hover:bg-white/3 hover:text-white/80'
                            "
                        >
                            <Icon :name="tab.icon" class="h-4.5 w-4.5 shrink-0" />
                            <span>{{ tab.label }}</span>
                        </button>
                    </template>
                </nav>

                <!-- Admin separator -->
                <template v-if="adminTabs.length > 0">
                    <div class="my-3 border-t border-white/6"></div>
                    <div
                        class="mb-2 px-3 text-[10px] font-semibold tracking-wider text-white/30
                            uppercase"
                    >
                        Admin
                    </div>

                    <nav class="space-y-0.5">
                        <template v-for="tab in adminTabs" :key="tab.id">
                            <!-- Tab with sub-tabs -->
                            <div v-if="tab.subTabs">
                                <button
                                    @click="setTab(tab)"
                                    class="group flex w-full items-center gap-2.5 rounded-lg px-3
                                        py-2 text-[13px] font-medium transition-all duration-150"
                                    :class="
                                        isTabActive(tab)
                                            ? 'bg-lava/10 text-lava shadow-[inset_3px_0_0_#ff4d4d]'
                                            : 'text-white/50 hover:bg-white/3 hover:text-white/80'
                                    "
                                >
                                    <Icon :name="tab.icon" class="h-4.5 w-4.5 shrink-0" />
                                    <span>{{ tab.label }}</span>
                                    <span
                                        @click="toggleExpand(tab, $event)"
                                        class="ml-auto cursor-pointer rounded p-0.5
                                            transition-colors hover:bg-white/10"
                                    >
                                        <Icon
                                            name="heroicons:chevron-down"
                                            class="h-4 w-4 transition-transform duration-200"
                                            :class="expandedTabs.has(tab.id) ? 'rotate-180' : ''"
                                        />
                                    </span>
                                </button>

                                <!-- Sub-tabs with collapse animation -->
                                <div
                                    class="overflow-hidden transition-all duration-200 ease-out"
                                    :class="expandedTabs.has(tab.id) ? 'max-h-75' : 'max-h-0'"
                                >
                                    <div class="mt-0.5 space-y-0.5 py-1">
                                        <button
                                            v-for="subTab in tab.subTabs"
                                            :key="subTab.id"
                                            @click="setSubTab(tab, subTab.id)"
                                            class="flex w-full items-center gap-2 rounded-md py-1.5
                                                pr-3 pl-11 text-xs font-medium transition-all
                                                duration-150"
                                            :class="
                                                isTabActive(tab) && isSubTabActive(subTab.id)
                                                    ? 'text-lava'
                                                    : `text-white/40 hover:bg-white/2
                                                        hover:text-white/70`
                                            "
                                        >
                                            <span
                                                class="h-1 w-1 rounded-full"
                                                :class="
                                                    isTabActive(tab) && isSubTabActive(subTab.id)
                                                        ? 'bg-lava'
                                                        : 'bg-white/20'
                                                "
                                            ></span>
                                            {{ subTab.label }}
                                        </button>
                                    </div>
                                </div>
                            </div>

                            <!-- Regular admin tab -->
                            <button
                                v-else
                                @click="setTab(tab)"
                                class="group flex w-full items-center gap-2.5 rounded-lg px-3 py-2
                                    text-[13px] font-medium transition-all duration-150"
                                :class="
                                    isTabActive(tab)
                                        ? 'bg-lava/10 text-lava shadow-[inset_3px_0_0_#ff4d4d]'
                                        : 'text-white/50 hover:bg-white/3 hover:text-white/80'
                                "
                            >
                                <Icon :name="tab.icon" class="h-4.5 w-4.5 shrink-0" />
                                <span>{{ tab.label }}</span>
                            </button>
                        </template>
                    </nav>
                </template>
            </div>
        </aside>

        <!-- Content area -->
        <main class="min-w-0 flex-1 overflow-y-auto p-4 sm:p-6 lg:p-8">
            <div class="mx-auto max-w-3xl">
                <SettingsAccount v-if="activeTab === 'account'" />
                <SettingsPlayer v-if="activeTab === 'player'" />
                <SettingsApp v-if="activeTab === 'app'" />
                <SettingsHomepage v-if="activeTab === 'homepage'" />
                <SettingsTags v-if="activeTab === 'tags'" />
                <SettingsParsingRules v-if="activeTab === 'parsing-rules'" />
                <SettingsUsers v-if="activeTab === 'users'" />
                <SettingsJobs v-if="activeTab === 'jobs'" :active-sub-tab="activeSubTab" />
                <SettingsStorage v-if="activeTab === 'storage'" />
                <SettingsTrash v-if="activeTab === 'trash'" />
            </div>
        </main>
    </div>
</template>
