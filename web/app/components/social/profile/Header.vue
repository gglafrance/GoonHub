<script setup lang="ts">
import type { Relationship, SocialListKind, SocialProfile } from '~/types/social';

// Top of a profile: identity, relation badges, follow/friend buttons, relation counts.
const props = defineProps<{ profile: SocialProfile }>();
const emit = defineEmits<{ update: [relationship: Relationship]; open: [list: SocialListKind] }>();

const counts = computed(() =>
    (
        [
            { kind: 'friends', label: 'Friends', value: props.profile.counts.friends },
            { kind: 'followers', label: 'Followers', value: props.profile.counts.followers },
            { kind: 'following', label: 'Following', value: props.profile.counts.following },
        ] as { kind: SocialListKind; label: string; value?: number }[]
    ).filter((c) => c.value !== undefined),
);
</script>

<template>
    <div class="glass-panel flex flex-col gap-4 p-5 sm:flex-row sm:items-center">
        <div class="relative shrink-0 self-start sm:self-auto">
            <SocialUserAvatar :username="profile.user.username" size="lg" />
            <SocialPresenceDot :user-id="profile.user.id" size="lg" />
        </div>
        <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
                <h1 class="truncate text-lg font-semibold text-white">
                    {{ profile.user.username }}
                </h1>
                <span
                    v-if="profile.relationship.self"
                    class="text-dim font-mono text-[9px] tracking-wider uppercase"
                >
                    You
                </span>
                <SocialRelationshipBadges :relationship="profile.relationship" />
            </div>
            <div class="text-dim mt-0.5 text-[11px]">
                Member since <UiFormattedDate :datetime="profile.user.created_at" />
            </div>
            <SocialPresenceBadge
                v-if="!profile.relationship.self"
                :user-id="profile.user.id"
                class="mt-1"
            />
            <div v-if="counts.length" class="mt-2 flex gap-4">
                <button
                    v-for="c in counts"
                    :key="c.kind"
                    class="group text-left"
                    @click="emit('open', c.kind)"
                >
                    <span class="group-hover:text-lava text-sm font-semibold text-white">
                        {{ c.value }}
                    </span>
                    <span class="text-dim ml-1 text-[11px]">{{ c.label }}</span>
                </button>
            </div>
        </div>
        <SocialRelationshipActions
            :user-id="profile.user.id"
            :username="profile.user.username"
            :relationship="profile.relationship"
            @update="emit('update', $event)"
        />
    </div>
</template>
