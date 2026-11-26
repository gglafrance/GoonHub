<script setup lang="ts">
import type { Profile, ProfileListKind } from '~/types/profile';

// Tabs of a profile page (Letterboxd style): the profile, then the full lists the viewer may
// open (each behind its privacy item and scenes:view, `profile.lists`).
const props = defineProps<{ profile: Profile }>();

const route = useRoute();

const tabs = computed(() => {
    const username = props.profile.user.username;
    const lists = props.profile.lists;
    const items: { id: 'profile' | ProfileListKind; label: string; icon: string }[] = [
        { id: 'profile', label: 'Profile', icon: 'heroicons:user' },
    ];
    if (lists?.liked) items.push({ id: 'likes', label: 'Likes', icon: 'heroicons:heart' });
    if (lists?.ratings) items.push({ id: 'ratings', label: 'Ratings', icon: 'heroicons:star' });
    if (lists?.jizz) {
        items.push({
            id: 'jizz',
            label: 'Jizz',
            icon: 'fluent-emoji-high-contrast:sweat-droplets',
        });
    }
    return items.map((t) => ({
        ...t,
        to: profileTabPath(username, t.id === 'profile' ? undefined : t.id),
    }));
});

// The last path segment names the tab (/u/<name> is the profile)
const active = computed(() => {
    const segment = route.path.split('/').filter(Boolean)[2];
    return segment ?? 'profile';
});
</script>

<template>
    <nav v-if="tabs.length > 1" class="border-border border-b" aria-label="Profile sections">
        <div class="scrollbar-none -mb-px flex gap-1 overflow-x-auto overflow-y-hidden">
            <NuxtLink
                v-for="tab in tabs"
                :key="tab.id"
                :to="tab.to"
                class="flex shrink-0 items-center gap-1.5 border-b-2 px-3 py-2 text-xs font-medium
                    transition-colors"
                :class="
                    active === tab.id
                        ? 'border-lava text-lava'
                        : 'text-dim border-transparent hover:text-white'
                "
                :aria-current="active === tab.id ? 'page' : undefined"
            >
                <Icon :name="tab.icon" size="14" />
                {{ tab.label }}
            </NuxtLink>
        </div>
    </nav>
</template>
