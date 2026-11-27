<script setup lang="ts">
import type { Profile } from '~/types/profile';
import type { Relationship, SocialListKind } from '~/types/social';

// Top of a profile: banner, avatar, display name and username, bio, join date, relation counts,
// relation badges and actions, and the owner's "Edit profile" toggle. The presence slot is for a
// presence badge (online / now watching) next to the name.
const props = defineProps<{ profile: Profile; editing?: boolean }>();
const emit = defineEmits<{
    update: [relationship: Relationship];
    open: [list: SocialListKind];
    edit: [];
}>();

defineSlots<{ presence?: () => unknown }>();

const user = computed(() => props.profile.user);
const name = computed(() => displayNameOf(user.value));
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
    <div class="glass-panel overflow-hidden">
        <div class="relative h-28 bg-gradient-to-br from-white/[0.04] to-transparent sm:h-44">
            <img
                v-if="profile.banner"
                :src="profile.banner.url"
                :srcset="`${profile.banner.url_md} 750w, ${profile.banner.url} 1500w`"
                sizes="(max-width: 768px) 750px, 1500px"
                alt=""
                class="h-full w-full object-cover"
            />
            <div class="from-void/90 absolute inset-0 bg-gradient-to-t via-transparent" />
        </div>
        <div class="relative px-4 pb-4 sm:px-5">
            <div class="-mt-10 flex flex-col gap-3 sm:-mt-12 sm:flex-row sm:items-end">
                <div class="relative shrink-0 self-start sm:self-auto">
                    <SocialUserAvatar
                        :username="user.username"
                        :display-name="user.display_name"
                        :user-id="user.id"
                        :avatar-version="user.avatar_version"
                        size="xl"
                        class="ring-void ring-4"
                    />
                    <SocialPresenceDot :user-id="user.id" size="lg" />
                </div>
                <div class="min-w-0 flex-1">
                    <div class="flex flex-wrap items-center gap-2">
                        <h1 class="truncate text-lg font-semibold text-white">{{ name }}</h1>
                        <slot name="presence">
                            <SocialPresenceBadge
                                v-if="!profile.relationship.self"
                                :user-id="user.id"
                            />
                        </slot>
                        <span
                            v-if="profile.relationship.self"
                            class="text-dim font-mono text-[9px] tracking-wider uppercase"
                        >
                            You
                        </span>
                        <SocialRelationshipBadges :relationship="profile.relationship" />
                    </div>
                    <div class="text-dim mt-0.5 flex flex-wrap gap-x-3 text-[11px]">
                        <span v-if="user.display_name" class="font-mono">@{{ user.username }}</span>
                        <span>Member since <UiFormattedDate :datetime="user.created_at" /></span>
                    </div>
                </div>
                <div class="flex items-center gap-2">
                    <button
                        v-if="profile.relationship.self"
                        class="border-border hover:border-lava/40 flex items-center gap-1.5
                            rounded-md border px-2.5 py-1.5 text-[11px] font-medium text-white
                            transition-colors"
                        :class="editing ? 'border-lava/40 bg-lava/10 text-lava' : ''"
                        @click="emit('edit')"
                    >
                        <Icon :name="editing ? 'heroicons:check' : 'heroicons:pencil'" size="13" />
                        {{ editing ? 'Done' : 'Edit profile' }}
                    </button>
                    <template v-else>
                        <SocialMessageButton
                            :username="user.username"
                            :relationship="profile.relationship"
                        />
                        <SocialRelationshipActions
                            :user-id="user.id"
                            :username="user.username"
                            :relationship="profile.relationship"
                            @update="emit('update', $event)"
                        />
                    </template>
                </div>
            </div>
            <ProfileBio v-if="profile.bio" :bio="profile.bio" class="mt-3 max-w-2xl" />
            <div v-if="counts.length" class="mt-3 flex gap-4">
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
    </div>
</template>
