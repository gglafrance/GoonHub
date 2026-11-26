<script setup lang="ts">
import type { ProfileBlockTypeInfo, ProfileLayoutBlock } from '~/types/profile';
import type { Audience, PrivacyItem, PrivacyOption } from '~/types/social';

// One block of the layout editor: shown or not, "Private" placeholder for those who may not
// see it, its options, and who can see it (the privacy item's audience, changed in place).
const props = defineProps<{
    block: ProfileLayoutBlock;
    info: ProfileBlockTypeInfo;
    privacy?: PrivacyItem;
    audiences: PrivacyOption<Audience>[];
    sharedWith: string[];
    first: boolean;
    last: boolean;
}>();
const emit = defineEmits<{
    update: [block: ProfileLayoutBlock];
    move: [delta: number];
    audience: [item: string, audience: Audience | 'default'];
}>();

function set(patch: Partial<ProfileLayoutBlock>) {
    emit('update', { ...props.block, ...patch });
}

function setOption(key: string, value: string | number) {
    set({ config: { ...props.block.config, [key]: value } });
}

function optionValue(key: string, def: number | string) {
    return props.block.config[key] ?? def;
}

const audienceOptions = computed(() =>
    props.audiences.map((a) => ({
        value: a.value,
        label: a.value === props.privacy?.default ? `${a.label} (default)` : a.label,
    })),
);

function onAudience(value: string | number) {
    if (!props.privacy) return;
    const v = String(value) as Audience;
    emit('audience', props.privacy.key, v === props.privacy.default ? 'default' : v);
}
</script>

<template>
    <div
        class="flex flex-col gap-2 p-3 sm:flex-row sm:items-center"
        :class="block.visible ? '' : 'opacity-60'"
    >
        <div class="flex min-w-0 flex-1 items-center gap-2">
            <Icon name="heroicons:bars-3" size="14" class="text-dim hidden cursor-grab sm:block" />
            <Icon
                :name="PROFILE_BLOCK_ICONS[block.type] ?? 'heroicons:squares-2x2'"
                size="14"
                class="text-lava"
            />
            <div class="min-w-0">
                <div class="text-xs font-medium text-white">{{ info.label }}</div>
                <div class="text-dim truncate text-[10px]">{{ info.description }}</div>
            </div>
        </div>
        <div class="flex flex-wrap items-center gap-2">
            <template v-for="opt in info.options" :key="opt.key">
                <input
                    v-if="opt.kind === 'int'"
                    type="number"
                    :min="opt.min"
                    :max="opt.max"
                    :title="opt.label"
                    :value="optionValue(opt.key, opt.default)"
                    class="border-border w-14 rounded-md border bg-white/[0.03] px-2 py-1
                        text-[11px] text-white"
                    @change="setOption(opt.key, Number(($event.target as HTMLInputElement).value))"
                />
                <div v-else class="w-32">
                    <UiSelectMenu
                        :model-value="optionValue(opt.key, opt.default)"
                        :options="opt.choices ?? []"
                        @update:model-value="setOption(opt.key, $event)"
                    />
                </div>
            </template>
            <div
                v-if="privacy"
                class="w-32"
                :title="
                    sharedWith.length
                        ? `Also applies to: ${sharedWith.join(', ')}`
                        : 'Who can see this block'
                "
            >
                <UiSelectMenu
                    :model-value="privacy.audience"
                    :options="audienceOptions"
                    @update:model-value="onAudience"
                />
            </div>
            <button
                class="flex items-center gap-1 text-[10px]"
                :class="block.locked_placeholder ? 'text-white' : 'text-dim'"
                title="Show a 'Private' placeholder to those who cannot see it"
                @click="set({ locked_placeholder: !block.locked_placeholder })"
            >
                <Icon name="heroicons:lock-closed" size="12" />
            </button>
            <UiToggle
                :model-value="block.visible"
                title="Shown on the profile"
                @update:model-value="set({ visible: $event })"
            />
            <div class="flex flex-col">
                <button
                    class="text-dim hover:text-white disabled:opacity-30"
                    :disabled="first"
                    title="Move up"
                    @click="emit('move', -1)"
                >
                    <Icon name="heroicons:chevron-up" size="12" />
                </button>
                <button
                    class="text-dim hover:text-white disabled:opacity-30"
                    :disabled="last"
                    title="Move down"
                    @click="emit('move', 1)"
                >
                    <Icon name="heroicons:chevron-down" size="12" />
                </button>
            </div>
        </div>
    </div>
</template>
