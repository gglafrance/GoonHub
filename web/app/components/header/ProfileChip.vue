<script setup lang="ts">
// The signed-in user (avatar, display name, role), link to their profile.
const authStore = useAuthStore();
const perms = useMyPermissions();
const socialStore = useSocialStore();
</script>

<template>
    <NuxtLink
        v-if="authStore.user"
        :to="profilePath(authStore.user.username)"
        class="group flex items-center gap-2"
        title="My profile"
    >
        <SocialUserAvatar
            :username="authStore.user.username"
            :display-name="socialStore.me?.display_name"
            :user-id="socialStore.me?.id"
            :avatar-version="socialStore.me?.avatar_version"
            size="xs"
            class="group-hover:border-lava/50 !h-7 !w-7"
        />
        <div class="hidden sm:block">
            <div class="group-hover:text-lava text-xs font-medium text-white">
                {{ socialStore.me?.display_name || authStore.user.username }}
            </div>
            <div
                class="font-mono text-[10px] tracking-wider uppercase"
                :class="perms.isSuperuser.value ? 'text-emerald' : 'text-dim'"
            >
                {{ authStore.user.role }}
            </div>
        </div>
    </NuxtLink>
</template>
