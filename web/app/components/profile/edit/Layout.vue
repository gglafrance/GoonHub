<script setup lang="ts">
import type { ProfileEdit, ProfileLayoutBlock } from '~/types/profile';
import type { Audience } from '~/types/social';

// Layout editor: the blocks in order (drag and drop, or the arrows), each shown or hidden,
// with its options and its audience. The audience is a privacy item, saved at once (several
// blocks may share one); the layout is saved with the button.
const props = defineProps<{ state: ProfileEdit }>();
const emit = defineEmits<{ saved: [] }>();

const api = useApiProfiles();
const socialApi = useApiSocial();
const blocks = ref<ProfileLayoutBlock[]>([]);
const privacy = ref(props.state.privacy);
const saving = ref(false);
const error = ref('');
const dirty = ref(false);

function sync() {
    blocks.value = props.state.layout.map((b) => ({ ...b, config: { ...b.config } }));
    privacy.value = props.state.privacy;
    dirty.value = false;
}
watch(() => props.state, sync);
onMounted(sync);

const infoOf = (type: string) => props.state.block_types.find((t) => t.type === type);
const privacyKeyOf = (type: string) => infoOf(type)?.privacy_item;
const privacyOf = (type: string) => privacy.value.items.find((i) => i.key === privacyKeyOf(type));
const labelOf = (type: string) => infoOf(type)?.label ?? type;

function move(from: number, to: number) {
    blocks.value = moveItem(blocks.value, from, to);
    dirty.value = true;
}
const drag = useDragReorder(move);

function update(i: number, block: ProfileLayoutBlock) {
    blocks.value[i] = block;
    dirty.value = true;
}

async function changeAudience(item: string, audience: Audience | 'default') {
    error.value = '';
    try {
        privacy.value = await socialApi.updatePrivacySettings({ items: { [item]: audience } });
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to change who can see it';
    }
}

async function persist(action: () => Promise<{ blocks: ProfileLayoutBlock[] }>) {
    saving.value = true;
    error.value = '';
    try {
        blocks.value = (await action()).blocks;
        dirty.value = false;
        emit('saved');
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to save the layout';
    } finally {
        saving.value = false;
    }
}
</script>

<template>
    <div class="space-y-3">
        <div class="border-border divide-border divide-y rounded-lg border">
            <div
                v-for="(block, i) in blocks"
                :key="block.type"
                v-bind="drag.handlers(i)"
                :class="drag.overIndex.value === i ? 'bg-lava/5' : ''"
            >
                <ProfileEditLayoutRow
                    v-if="infoOf(block.type)"
                    :block="block"
                    :info="infoOf(block.type)!"
                    :privacy="privacyOf(block.type)"
                    :audiences="privacy.audiences"
                    :shared-with="
                        blocksSharingPrivacy(blocks, privacyKeyOf, block.type).map(labelOf)
                    "
                    :first="i === 0"
                    :last="i === blocks.length - 1"
                    @update="update(i, $event)"
                    @move="move(i, i + $event)"
                    @audience="changeAudience"
                />
            </div>
        </div>
        <p class="text-dim text-[10px]">
            Who can see a block is a privacy setting (also in Settings > Privacy); blocks of the
            same kind share it.
        </p>
        <div class="flex items-center gap-3">
            <button
                class="bg-lava hover:bg-lava/90 rounded-md px-3 py-1.5 text-xs font-medium
                    text-white disabled:opacity-50"
                :disabled="!dirty || saving"
                @click="persist(() => api.setLayout(blocks))"
            >
                {{ saving ? 'Saving...' : 'Save layout' }}
            </button>
            <button
                class="text-dim text-[11px] hover:text-white"
                :disabled="saving"
                @click="persist(api.resetLayout)"
            >
                Reset to default
            </button>
            <span v-if="error" class="text-lava text-[11px]">{{ error }}</span>
        </div>
    </div>
</template>
