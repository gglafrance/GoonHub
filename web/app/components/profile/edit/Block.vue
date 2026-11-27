<script setup lang="ts">
import type { ProfileEdit, ProfileLayoutBlock } from '~/types/profile';
import type { Audience, PrivacySettings } from '~/types/social';

// A block in the section editor: shown or not, "Private" placeholder, moves (up, down, to the
// column on the left or right), removal, and its settings (options, who can see it). A custom
// list block picks one of the owner's lists and shows what the list depends on.
const props = defineProps<{
    block: ProfileLayoutBlock;
    state: ProfileEdit;
    privacy: PrivacySettings;
    can: { up: boolean; down: boolean; left: boolean; right: boolean };
}>();
const emit = defineEmits<{
    update: [block: ProfileLayoutBlock];
    remove: [];
    move: [direction: 'up' | 'down' | 'left' | 'right'];
    audience: [item: string, audience: Audience | 'default'];
}>();

const open = ref(false);
const info = computed(() => props.state.block_types.find((t) => t.type === props.block.type));
const isList = computed(() => props.block.type === 'custom_list');
const list = computed(() =>
    isList.value
        ? props.state.lists.find((l) => l.id === Number(props.block.config.list_id))
        : undefined,
);
const label = computed(() => list.value?.title ?? info.value?.label ?? props.block.type);
const privacyItem = computed(() =>
    isList.value ? undefined : props.privacy.items.find((i) => i.key === info.value?.privacy_item),
);
const audienceLabel = (a: Audience) =>
    props.privacy.audiences.find((x) => x.value === a)?.label ?? a;

const audienceOptions = computed(() =>
    props.privacy.audiences.map((a) => ({
        value: a.value,
        label: a.value === privacyItem.value?.default ? `${a.label} (default)` : a.label,
    })),
);
const listOptions = computed(() => props.state.lists.map((l) => ({ value: l.id, label: l.title })));

function set(patch: Partial<ProfileLayoutBlock>) {
    emit('update', { ...props.block, ...patch });
}

function setOption(key: string, value: string | number) {
    set({ config: { ...props.block.config, [key]: value } });
}

function onAudience(value: string | number) {
    const item = privacyItem.value;
    if (!item) return;
    const v = String(value) as Audience;
    emit('audience', item.key, v === item.default ? 'default' : v);
}
</script>

<template>
    <div
        class="border-border rounded-md border bg-white/[0.02] text-xs"
        :class="block.visible ? '' : 'opacity-60'"
    >
        <div class="flex items-center gap-1.5 px-2 py-1.5">
            <Icon name="heroicons:bars-3" size="12" class="text-dim shrink-0 cursor-grab" />
            <Icon
                :name="PROFILE_BLOCK_ICONS[block.type] ?? 'heroicons:squares-2x2'"
                size="13"
                class="text-lava shrink-0"
            />
            <span class="min-w-0 flex-1 truncate font-medium text-white" :title="info?.description">
                {{ label }}
            </span>
            <button
                class="text-dim hover:text-white"
                :class="open ? 'text-white' : ''"
                title="Settings"
                :aria-expanded="open"
                @click="open = !open"
            >
                <Icon name="heroicons:cog-6-tooth" size="13" />
            </button>
            <UiToggle
                :model-value="block.visible"
                title="Shown on the profile"
                @update:model-value="set({ visible: $event })"
            />
        </div>
        <div class="text-dim flex items-center gap-1 border-t border-white/5 px-2 py-1">
            <button
                v-for="dir in ['up', 'down', 'left', 'right'] as const"
                :key="dir"
                class="hover:text-white disabled:opacity-25"
                :disabled="!props.can[dir]"
                :title="`Move ${dir}`"
                @click="emit('move', dir)"
            >
                <Icon :name="`heroicons:arrow-${dir}`" size="11" />
            </button>
            <button
                class="ml-1"
                :class="block.locked_placeholder ? 'text-white' : ''"
                title="Show a 'Private' placeholder to those who cannot see it"
                @click="set({ locked_placeholder: !block.locked_placeholder })"
            >
                <Icon name="heroicons:lock-closed" size="11" />
            </button>
            <button
                class="hover:text-lava ml-auto"
                title="Remove the block"
                @click="emit('remove')"
            >
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </div>
        <div v-if="open" class="space-y-2 border-t border-white/5 p-2">
            <template v-for="opt in info?.options ?? []" :key="opt.key">
                <label class="text-dim block text-[10px]">
                    {{ opt.label }}
                    <UiSelectMenu
                        v-if="opt.key === 'list_id'"
                        :model-value="Number(block.config.list_id ?? 0)"
                        :options="listOptions"
                        @update:model-value="setOption('list_id', Number($event))"
                    />
                    <input
                        v-else-if="opt.kind === 'int'"
                        type="number"
                        :min="opt.min"
                        :max="opt.max"
                        :value="block.config[opt.key] ?? opt.default"
                        class="border-border mt-0.5 w-full rounded-md border bg-white/[0.03] px-2
                            py-1 text-[11px] text-white"
                        @change="
                            setOption(opt.key, Number(($event.target as HTMLInputElement).value))
                        "
                    />
                    <UiSelectMenu
                        v-else
                        :model-value="block.config[opt.key] ?? opt.default"
                        :options="opt.choices ?? []"
                        @update:model-value="setOption(opt.key, $event)"
                    />
                </label>
            </template>
            <label v-if="privacyItem" class="text-dim block text-[10px]">
                Who can see it
                <UiSelectMenu
                    :model-value="privacyItem.audience"
                    :options="audienceOptions"
                    @update:model-value="onAudience"
                />
            </label>
            <div v-else-if="list" class="text-dim space-y-1 text-[10px]">
                <div>
                    List audience:
                    <span class="text-white">{{ audienceLabel(list.audience) }}</span>
                </div>
                <div v-if="list.dependencies.length">
                    Also needs:
                    <span v-for="d in list.dependencies" :key="d.key" class="mr-1 text-white">
                        {{ d.label }} ({{ audienceLabel(d.audience) }})
                    </span>
                </div>
                <div>Edit the list in Custom lists.</div>
            </div>
        </div>
    </div>
</template>
