<script setup lang="ts">
import type { Actor } from '~/types/actor';

const props = defineProps<{
    visible: boolean;
    actor?: Actor | null;
    initialName?: string;
}>();

const emit = defineEmits<{
    close: [];
    updated: [];
    created: [actor: Actor];
}>();

const api = useApi();

const isEditMode = computed(() => !!props.actor);

type ModalView = 'form' | 'porndb-search';
const currentView = ref<ModalView>('form');

const form = ref({
    name: '',
    aliases: [] as string[],
    image_url: '',
    gender: '',
    birthday: '',
    date_of_death: '',
    astrology: '',
    birthplace: '',
    ethnicity: '',
    nationality: '',
    career_start_year: null as number | null,
    career_end_year: null as number | null,
    height_cm: null as number | null,
    weight_kg: null as number | null,
    measurements: '',
    cupsize: '',
    hair_color: '',
    eye_color: '',
    tattoos: '',
    piercings: '',
    fake_boobs: false,
    same_sex_only: false,
});

const newAlias = ref('');

const loading = ref(false);
const error = ref('');
const imageFile = ref<File | null>(null);
const imagePreview = ref<string | null>(null);

const searchName = computed(() => form.value.name || props.actor?.name || props.initialName || '');

const addAlias = () => {
    const alias = newAlias.value.trim();
    if (alias && !form.value.aliases.includes(alias)) {
        form.value.aliases.push(alias);
    }
    newAlias.value = '';
};

const removeAlias = (index: number) => {
    form.value.aliases.splice(index, 1);
};

const revokeBlobPreview = () => {
    if (imagePreview.value && imagePreview.value.startsWith('blob:')) {
        URL.revokeObjectURL(imagePreview.value);
    }
};

const resetForm = () => {
    form.value = {
        name: props.initialName || '',
        aliases: [],
        image_url: '',
        gender: '',
        birthday: '',
        date_of_death: '',
        astrology: '',
        birthplace: '',
        ethnicity: '',
        nationality: '',
        career_start_year: null,
        career_end_year: null,
        height_cm: null,
        weight_kg: null,
        measurements: '',
        cupsize: '',
        hair_color: '',
        eye_color: '',
        tattoos: '',
        piercings: '',
        fake_boobs: false,
        same_sex_only: false,
    };
    newAlias.value = '';
    revokeBlobPreview();
    imagePreview.value = null;
};

const syncForm = () => {
    if (!props.actor) {
        resetForm();
        return;
    }
    form.value = {
        name: props.actor.name,
        aliases: [...(props.actor.aliases || [])],
        image_url: props.actor.image_url || '',
        gender: props.actor.gender || '',
        birthday: props.actor.birthday?.split('T')[0] ?? '',
        date_of_death: props.actor.date_of_death?.split('T')[0] ?? '',
        astrology: props.actor.astrology || '',
        birthplace: props.actor.birthplace || '',
        ethnicity: props.actor.ethnicity || '',
        nationality: props.actor.nationality || '',
        career_start_year: props.actor.career_start_year || null,
        career_end_year: props.actor.career_end_year || null,
        height_cm: props.actor.height_cm || null,
        weight_kg: props.actor.weight_kg || null,
        measurements: props.actor.measurements || '',
        cupsize: props.actor.cupsize || '',
        hair_color: props.actor.hair_color || '',
        eye_color: props.actor.eye_color || '',
        tattoos: props.actor.tattoos || '',
        piercings: props.actor.piercings || '',
        fake_boobs: props.actor.fake_boobs,
        same_sex_only: props.actor.same_sex_only,
    };
    imagePreview.value = props.actor.image_url || null;
};

watch(
    () => props.actor,
    () => {
        syncForm();
    },
    { immediate: true },
);

watch(
    () => props.visible,
    (visible) => {
        if (visible) {
            currentView.value = 'form';
            if (!props.actor) {
                resetForm();
            }
        }
    },
);

const handleImageChange = (event: Event) => {
    const input = event.target as HTMLInputElement;
    if (input.files && input.files[0]) {
        revokeBlobPreview();
        imageFile.value = input.files[0];
        imagePreview.value = URL.createObjectURL(input.files[0]);
        form.value.image_url = '';
    }
};

const handleSubmit = async () => {
    error.value = '';
    loading.value = true;

    try {
        const payload: Record<string, string | number | boolean | null | string[]> = {
            ...form.value,
            birthday: form.value.birthday || null,
            date_of_death: form.value.date_of_death || null,
        };

        if (isEditMode.value && props.actor) {
            if (imageFile.value) {
                await api.uploadActorImage(props.actor.id, imageFile.value);
                delete payload.image_url;
            }
            await api.updateActor(props.actor.id, payload);
            emit('updated');
        } else {
            let newActor = await api.createActor(payload);
            if (imageFile.value && newActor.id) {
                newActor = await api.uploadActorImage(newActor.id, imageFile.value);
            }
            emit('created', newActor);
        }
    } catch (e: unknown) {
        error.value =
            e instanceof Error
                ? e.message
                : `Failed to ${isEditMode.value ? 'update' : 'create'} actor`;
    } finally {
        loading.value = false;
    }
};

const handleClose = () => {
    error.value = '';
    revokeBlobPreview();
    imageFile.value = null;
    emit('close');
};

const handlePornDBApply = (data: Record<string, unknown>) => {
    for (const [key, value] of Object.entries(data)) {
        if (key in form.value) {
            (form.value as Record<string, unknown>)[key] = value;
        }
    }
    if (data.image_url) {
        imagePreview.value = data.image_url as string;
    }
    currentView.value = 'form';
};

const genderOptions = [
    { value: '', label: 'Not specified' },
    { value: 'Female', label: 'Female' },
    { value: 'Male', label: 'Male' },
    { value: 'Trans', label: 'Trans' },
    { value: 'Non-binary', label: 'Non-binary' },
];
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/70
                p-4 backdrop-blur-sm"
            @click.self="handleClose"
        >
            <div class="glass-panel border-border my-8 w-full max-w-2xl border p-6">
                <ActorsEditmodalFormView
                    v-if="currentView === 'form'"
                    :form="form"
                    :is-edit-mode="isEditMode"
                    :loading="loading"
                    :error="error"
                    :image-preview="imagePreview"
                    :new-alias="newAlias"
                    :gender-options="genderOptions"
                    @submit="handleSubmit"
                    @close="handleClose"
                    @fetch-porndb="currentView = 'porndb-search'"
                    @image-change="handleImageChange"
                    @add-alias="addAlias"
                    @remove-alias="removeAlias"
                    @update:new-alias="newAlias = $event"
                />

                <ActorsEditmodalPornDBSearch
                    v-else
                    :actor-name="searchName"
                    :form="form"
                    @apply="handlePornDBApply"
                    @back="currentView = 'form'"
                />
            </div>
        </div>
    </Teleport>
</template>
