<script setup lang="ts">
import type { SocialListKind } from '~/types/social';

// Friends page: tabs Friends, Requests, Following, Followers, Find people (?tab=).
type TabId = 'friends' | 'requests' | 'following' | 'followers' | 'find';

const authStore = useAuthStore();
const socialStore = useSocialStore();
const api = useApiSocial();
const route = useRoute();
const router = useRouter();

useHead({ title: 'Friends' });

definePageMeta({
    middleware: ['auth'],
});

const tabs: { id: TabId; label: string; icon: string }[] = [
    { id: 'friends', label: 'Friends', icon: 'heroicons:users' },
    { id: 'requests', label: 'Requests', icon: 'heroicons:inbox-arrow-down' },
    { id: 'following', label: 'Following', icon: 'heroicons:arrow-up-right' },
    { id: 'followers', label: 'Followers', icon: 'heroicons:arrow-down-left' },
    { id: 'find', label: 'Find people', icon: 'heroicons:magnifying-glass' },
];

const activeTab = computed<TabId>(() => {
    const t = route.query.tab as TabId;
    return tabs.some((x) => x.id === t) ? t : 'friends';
});

function setTab(id: TabId) {
    router.replace({ query: { ...route.query, tab: id } });
}

const me = computed(() => authStore.user?.username ?? '');
const listKind = computed<SocialListKind | null>(() =>
    activeTab.value === 'friends' ||
    activeTab.value === 'following' ||
    activeTab.value === 'followers'
        ? activeTab.value
        : null,
);
const listLoader = (page: number) => api.listRelations(me.value, listKind.value!, page);

const emptyTexts: Record<SocialListKind, string> = {
    friends: 'No friend yet. Find people and send them a friend request.',
    following: 'You do not follow anyone yet.',
    followers: 'Nobody follows you yet.',
};

// Keep a row only while it still belongs to my list
const keepIf = computed(() => {
    switch (listKind.value) {
        case 'friends':
            return (rel: { friend: boolean }) => rel.friend;
        case 'following':
            return (rel: { following: boolean }) => rel.following;
        default:
            return undefined;
    }
});
</script>

<template>
    <div class="mx-auto max-w-3xl px-4 py-6 sm:px-5">
        <div class="mb-4 flex items-center justify-between">
            <h1 class="text-lg font-semibold text-white">Friends</h1>
            <NuxtLink
                v-if="me"
                :to="profilePath(me)"
                class="text-dim hover:text-lava text-[11px] transition-colors"
            >
                My profile
            </NuxtLink>
        </div>

        <!-- The border is on the outer box and the scroller overlaps it by 1px, so the active tab
             underline covers the border without any vertical overflow (no stray scrollbar) -->
        <div class="border-border mb-4 border-b">
            <div class="scrollbar-none -mb-px flex gap-1 overflow-x-auto overflow-y-hidden">
                <button
                    v-for="tab in tabs"
                    :key="tab.id"
                    class="flex shrink-0 items-center gap-1.5 border-b-2 px-3 py-2 text-xs
                        font-medium transition-colors"
                    :class="
                        activeTab === tab.id
                            ? 'border-lava text-lava'
                            : 'text-dim border-transparent hover:text-white'
                    "
                    @click="setTab(tab.id)"
                >
                    <Icon :name="tab.icon" size="14" />
                    {{ tab.label }}
                    <span
                        v-if="tab.id === 'requests' && socialStore.incomingRequests > 0"
                        class="bg-lava rounded-full px-1.5 font-mono text-[9px] text-white"
                    >
                        {{ badgeCount(socialStore.incomingRequests) }}
                    </span>
                </button>
            </div>
        </div>

        <SocialUserList
            v-if="listKind && me"
            :key="listKind"
            :loader="listLoader"
            :reload-key="socialStore.version"
            :keep-if="keepIf"
            :since-label="listKind === 'friends' ? 'Friends since' : 'Since'"
            :empty-text="emptyTexts[listKind]"
        />
        <SocialFriendsRequestsTab v-else-if="activeTab === 'requests'" />
        <SocialFriendsFindPeople v-else-if="activeTab === 'find'" />
    </div>
</template>
