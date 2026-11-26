<script setup lang="ts">
import type { SocialListKind, SocialProfile } from '~/types/social';

// Friends / followers / following of a profile, each shown only when its privacy allows it.
const props = defineProps<{ profile: SocialProfile; reloadKey?: number }>();
const active = defineModel<SocialListKind>('active', { default: 'friends' });

const api = useApiSocial();

const tabs = computed(() => [
    { id: 'friends' as const, label: 'Friends', visible: props.profile.visibility.friends_list },
    {
        id: 'followers' as const,
        label: 'Followers',
        visible: props.profile.visibility.followers_list,
    },
    {
        id: 'following' as const,
        label: 'Following',
        visible: props.profile.visibility.followers_list,
    },
]);
const current = computed(() => tabs.value.find((t) => t.id === active.value));
const loader = (page: number) => api.listRelations(props.profile.user.username, active.value, page);
</script>

<template>
    <div class="glass-panel p-4">
        <div class="mb-3 flex gap-1">
            <button
                v-for="tab in tabs"
                :key="tab.id"
                class="flex items-center gap-1 rounded-md px-2.5 py-1 text-[11px] font-medium
                    transition-all"
                :class="
                    active === tab.id
                        ? 'bg-lava/10 text-lava'
                        : 'text-dim hover:bg-white/5 hover:text-white'
                "
                @click="active = tab.id"
            >
                <Icon v-if="!tab.visible" name="heroicons:lock-closed" size="11" />
                {{ tab.label }}
            </button>
        </div>
        <SocialUserList
            v-if="current?.visible"
            :key="active"
            :loader="loader"
            :reload-key="reloadKey"
            empty-text="Nobody here yet."
        />
        <p
            v-else
            class="text-dim border-border rounded-lg border border-dashed px-3 py-6 text-center
                text-xs"
        >
            <Icon name="heroicons:lock-closed" size="12" class="mr-1" />
            {{ profile.user.username }} keeps this list private.
        </p>
    </div>
</template>
