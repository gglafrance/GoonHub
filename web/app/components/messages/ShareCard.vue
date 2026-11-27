<script setup lang="ts">
import type { MessageAttachment } from '~/types/messages';

// Card of a shared video, marker, playlist or lobby invite, as resolved for me by the server
// (never any file information). Opens the watch page (at the shared time, or the marker
// range), the playlist page, or joins the lobby. "Not available" when I may not see it.
const props = defineProps<{ attachment: MessageAttachment }>();
const emit = defineEmits<{ join: [] }>();

const info = computed(() => shareCardInfo(props.attachment));
const link = computed(() => shareCardLink(props.attachment));
const lobby = computed(() => props.attachment.lobby);
const thumbFailed = ref(false);
// Resolved in setup: resolveComponent does not work from a template expression (the card was
// never rendered as a link, so it could not be opened)
const NuxtLinkComponent = resolveComponent('NuxtLink');
</script>

<template>
    <div
        v-if="!attachment.available"
        class="border-border text-dim flex items-center gap-2 rounded-lg border border-dashed px-3
            py-2 text-[11px]"
    >
        <Icon name="heroicons:no-symbol" size="13" class="shrink-0" />
        {{ unavailableShareText(attachment) }}
    </div>
    <component
        :is="link ? NuxtLinkComponent : 'div'"
        v-else
        :to="link ?? undefined"
        class="group/card border-border bg-void/60 hover:border-lava/30 flex w-72 max-w-full
            overflow-hidden rounded-lg border transition-colors"
    >
        <div class="relative aspect-video w-28 shrink-0 overflow-hidden bg-black/50">
            <img
                v-if="info.thumbnail && !thumbFailed"
                :src="info.thumbnail"
                alt=""
                class="h-full w-full object-cover"
                loading="lazy"
                @error="thumbFailed = true"
            />
            <div v-else class="text-dim flex h-full w-full items-center justify-center">
                <Icon :name="info.icon" size="18" />
            </div>
            <span
                v-if="info.badge"
                class="absolute right-1 bottom-1 rounded bg-black/75 px-1 py-px font-mono text-[9px]
                    text-white"
            >
                {{ info.badge }}
            </span>
        </div>
        <div class="flex min-w-0 flex-1 flex-col justify-center gap-0.5 px-2.5 py-1.5">
            <span
                class="text-lava flex items-center gap-1 text-[9px] font-semibold tracking-wider
                    uppercase"
            >
                <Icon :name="info.icon" size="10" />
                {{ info.kindLabel }}
            </span>
            <span
                class="group-hover/card:text-lava truncate text-xs font-medium text-white
                    transition-colors"
            >
                {{ info.title }}
            </span>
            <span class="text-dim truncate text-[10px]">{{ info.subtitle }}</span>
            <button
                v-if="lobby"
                class="bg-lava hover:bg-lava-glow mt-1 self-start rounded-md px-2.5 py-0.5
                    text-[10px] font-semibold text-white transition-colors"
                @click.prevent="emit('join')"
            >
                {{ lobby.is_member ? 'Open' : 'Join' }}
            </button>
        </div>
    </component>
</template>
