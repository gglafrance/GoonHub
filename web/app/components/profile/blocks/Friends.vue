<script setup lang="ts">
import type { ProfileBlock, ProfileFriendsPayload } from '~/types/profile';

// friends: a preview of the owner's friends (profile block, behind friends_list).
const props = defineProps<{ block: ProfileBlock<ProfileFriendsPayload>; owner: boolean }>();
const items = computed(() => props.block.widget?.items ?? []);
</script>

<template>
    <div class="grid grid-cols-4 gap-3 sm:grid-cols-6 lg:grid-cols-8">
        <NuxtLink
            v-for="f in items"
            :key="f.id"
            :to="profilePath(f.username)"
            class="group flex min-w-0 flex-col items-center gap-1.5"
            :title="f.username"
        >
            <SocialUserAvatar
                :username="f.username"
                :display-name="f.display_name"
                :user-id="f.id"
                :avatar-version="f.avatar_version"
                size="md"
                class="group-hover:border-lava/50 transition-colors"
            />
            <span class="group-hover:text-lava w-full truncate text-center text-[11px] text-white">
                {{ f.display_name || f.username }}
            </span>
        </NuxtLink>
    </div>
</template>
