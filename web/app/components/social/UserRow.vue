<script setup lang="ts">
import type { Relationship, SocialUserEntry } from '~/types/social';

// One user of a social list: avatar, linked username, badges, relation date, actions.
const props = defineProps<{ entry: SocialUserEntry; sinceLabel?: string }>();
const emit = defineEmits<{ update: [relationship: Relationship] }>();
</script>

<template>
    <div
        class="border-border hover:border-border-hover flex items-center gap-3 rounded-lg border
            bg-white/[0.02] px-3 py-2 transition-colors"
    >
        <NuxtLink :to="profilePath(props.entry.username)" class="relative shrink-0">
            <SocialUserAvatar
                :username="props.entry.username"
                :display-name="props.entry.display_name"
                :user-id="props.entry.id"
                :avatar-version="props.entry.avatar_version"
            />
            <SocialPresenceDot :user-id="props.entry.id" />
        </NuxtLink>
        <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
                <NuxtLink
                    :to="profilePath(props.entry.username)"
                    class="hover:text-lava truncate text-sm font-medium text-white
                        transition-colors"
                >
                    {{ displayNameOf(props.entry) }}
                </NuxtLink>
                <span
                    v-if="props.entry.display_name"
                    class="text-dim truncate font-mono text-[10px]"
                >
                    @{{ props.entry.username }}
                </span>
                <span
                    v-if="props.entry.relationship.self"
                    class="text-dim font-mono text-[9px] tracking-wider uppercase"
                >
                    You
                </span>
                <SocialRelationshipBadges :relationship="props.entry.relationship" />
            </div>
            <div v-if="props.sinceLabel" class="text-dim text-[11px]">
                {{ props.sinceLabel }}
                <NuxtTime :datetime="props.entry.since" relative />
            </div>
        </div>
        <slot name="actions">
            <SocialRelationshipActions
                :user-id="props.entry.id"
                :username="props.entry.username"
                :relationship="props.entry.relationship"
                @update="emit('update', $event)"
            />
        </slot>
    </div>
</template>
