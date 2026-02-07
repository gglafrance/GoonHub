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
        if (visible && !props.actor) {
            resetForm();
        }
    },
);

const handleImageChange = (event: Event) => {
    const input = event.target as HTMLInputElement;
    if (input.files && input.files[0]) {
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
            // Upload image first if provided — remove image_url from payload
            // so the updateActor call doesn't overwrite the URL set by the upload
            if (imageFile.value) {
                await api.uploadActorImage(props.actor.id, imageFile.value);
                delete payload.image_url;
            }
            await api.updateActor(props.actor.id, payload);
            emit('updated');
        } else {
            // Create new actor
            let newActor = await api.createActor(payload);
            // Upload image if provided — use the returned actor which has image_url set
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
    imageFile.value = null;
    emit('close');
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
                <h3 class="mb-4 text-sm font-semibold text-white">
                    {{ isEditMode ? 'Edit Actor' : 'Create Actor' }}
                </h3>

                <div
                    v-if="error"
                    class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2
                        text-xs"
                >
                    {{ error }}
                </div>

                <form class="space-y-4" @submit.prevent="handleSubmit">
                    <!-- Image upload -->
                    <div class="flex items-start gap-4">
                        <div
                            class="bg-surface border-border relative h-32 w-24 shrink-0
                                overflow-hidden rounded-lg border"
                        >
                            <img
                                v-if="imagePreview"
                                :src="imagePreview"
                                class="h-full w-full object-cover"
                                alt="Actor preview"
                            />
                            <div
                                v-else
                                class="text-dim flex h-full w-full items-center justify-center"
                            >
                                <Icon name="heroicons:user" size="32" />
                            </div>
                        </div>
                        <div class="flex-1">
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Image
                            </label>
                            <input
                                type="file"
                                accept="image/*"
                                class="border-border bg-void/80 file:bg-panel file:text-dim w-full
                                    rounded-lg border px-3 py-2 text-sm text-white file:mr-3
                                    file:rounded-lg file:border-0 file:px-3 file:py-1 file:text-xs"
                                @change="handleImageChange"
                            />
                            <div class="mt-2">
                                <label
                                    class="text-dim mb-1 block text-[11px] font-medium
                                        tracking-wider uppercase"
                                >
                                    Or Image URL
                                </label>
                                <input
                                    v-model="form.image_url"
                                    type="url"
                                    class="border-border bg-void/80 placeholder-dim/50
                                        focus:border-lava/40 focus:ring-lava/20 w-full rounded-lg
                                        border px-3 py-2 text-sm text-white transition-all
                                        focus:ring-1 focus:outline-none"
                                    placeholder="https://..."
                                />
                            </div>
                        </div>
                    </div>

                    <!-- Basic Info -->
                    <div class="grid grid-cols-2 gap-3">
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Name *
                            </label>
                            <input
                                v-model="form.name"
                                type="text"
                                required
                                class="border-border bg-void/80 placeholder-dim/50
                                    focus:border-lava/40 focus:ring-lava/20 w-full rounded-lg border
                                    px-3 py-2 text-sm text-white transition-all focus:ring-1
                                    focus:outline-none"
                            />
                        </div>
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Gender
                            </label>
                            <select
                                v-model="form.gender"
                                class="border-border bg-void/80 focus:border-lava/40
                                    focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm
                                    text-white transition-all focus:ring-1 focus:outline-none"
                            >
                                <option
                                    v-for="opt in genderOptions"
                                    :key="opt.value"
                                    :value="opt.value"
                                >
                                    {{ opt.label }}
                                </option>
                            </select>
                        </div>
                    </div>

                    <!-- Aliases -->
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Aliases
                        </label>
                        <div class="flex gap-2">
                            <input
                                v-model="newAlias"
                                type="text"
                                placeholder="Add alias..."
                                class="border-border bg-void/80 placeholder-dim/50
                                    focus:border-lava/40 focus:ring-lava/20 flex-1 rounded-lg border
                                    px-3 py-2 text-sm text-white transition-all focus:ring-1
                                    focus:outline-none"
                                @keyup.enter="addAlias"
                            />
                            <button
                                type="button"
                                :disabled="!newAlias.trim()"
                                class="border-border bg-panel hover:border-lava/50 hover:text-lava
                                    text-dim shrink-0 rounded-lg border px-3 py-2 text-sm
                                    transition-colors disabled:cursor-not-allowed
                                    disabled:opacity-40"
                                @click="addAlias"
                            >
                                Add
                            </button>
                        </div>
                        <div v-if="form.aliases.length > 0" class="mt-2 flex flex-wrap gap-1.5">
                            <span
                                v-for="(alias, index) in form.aliases"
                                :key="index"
                                class="border-border bg-surface inline-flex items-center gap-1
                                    rounded-full border px-2.5 py-1 text-xs text-white"
                            >
                                {{ alias }}
                                <button
                                    type="button"
                                    class="text-dim hover:text-lava transition-colors"
                                    @click="removeAlias(index)"
                                >
                                    <Icon name="heroicons:x-mark" size="12" />
                                </button>
                            </span>
                        </div>
                    </div>

                    <!-- Demographics -->
                    <div class="grid grid-cols-2 gap-3">
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Birthday
                            </label>
                            <input
                                v-model="form.birthday"
                                type="date"
                                class="border-border bg-void/80 focus:border-lava/40
                                    focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm
                                    text-white transition-all focus:ring-1 focus:outline-none"
                            />
                        </div>
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Nationality
                            </label>
                            <input
                                v-model="form.nationality"
                                type="text"
                                class="border-border bg-void/80 placeholder-dim/50
                                    focus:border-lava/40 focus:ring-lava/20 w-full rounded-lg border
                                    px-3 py-2 text-sm text-white transition-all focus:ring-1
                                    focus:outline-none"
                            />
                        </div>
                    </div>

                    <div class="grid grid-cols-2 gap-3">
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Birthplace
                            </label>
                            <input
                                v-model="form.birthplace"
                                type="text"
                                class="border-border bg-void/80 placeholder-dim/50
                                    focus:border-lava/40 focus:ring-lava/20 w-full rounded-lg border
                                    px-3 py-2 text-sm text-white transition-all focus:ring-1
                                    focus:outline-none"
                            />
                        </div>
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Ethnicity
                            </label>
                            <input
                                v-model="form.ethnicity"
                                type="text"
                                class="border-border bg-void/80 placeholder-dim/50
                                    focus:border-lava/40 focus:ring-lava/20 w-full rounded-lg border
                                    px-3 py-2 text-sm text-white transition-all focus:ring-1
                                    focus:outline-none"
                            />
                        </div>
                    </div>

                    <!-- Physical -->
                    <div class="grid grid-cols-4 gap-3">
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Height (cm)
                            </label>
                            <input
                                v-model.number="form.height_cm"
                                type="number"
                                min="0"
                                max="300"
                                class="border-border bg-void/80 focus:border-lava/40
                                    focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm
                                    text-white transition-all focus:ring-1 focus:outline-none"
                            />
                        </div>
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Weight (kg)
                            </label>
                            <input
                                v-model.number="form.weight_kg"
                                type="number"
                                min="0"
                                max="300"
                                class="border-border bg-void/80 focus:border-lava/40
                                    focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm
                                    text-white transition-all focus:ring-1 focus:outline-none"
                            />
                        </div>
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Measurements
                            </label>
                            <input
                                v-model="form.measurements"
                                type="text"
                                placeholder="34-24-34"
                                class="border-border bg-void/80 placeholder-dim/50
                                    focus:border-lava/40 focus:ring-lava/20 w-full rounded-lg border
                                    px-3 py-2 text-sm text-white transition-all focus:ring-1
                                    focus:outline-none"
                            />
                        </div>
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Cup Size
                            </label>
                            <input
                                v-model="form.cupsize"
                                type="text"
                                class="border-border bg-void/80 placeholder-dim/50
                                    focus:border-lava/40 focus:ring-lava/20 w-full rounded-lg border
                                    px-3 py-2 text-sm text-white transition-all focus:ring-1
                                    focus:outline-none"
                            />
                        </div>
                    </div>

                    <div class="grid grid-cols-2 gap-3">
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Hair Color
                            </label>
                            <input
                                v-model="form.hair_color"
                                type="text"
                                class="border-border bg-void/80 placeholder-dim/50
                                    focus:border-lava/40 focus:ring-lava/20 w-full rounded-lg border
                                    px-3 py-2 text-sm text-white transition-all focus:ring-1
                                    focus:outline-none"
                            />
                        </div>
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Eye Color
                            </label>
                            <input
                                v-model="form.eye_color"
                                type="text"
                                class="border-border bg-void/80 placeholder-dim/50
                                    focus:border-lava/40 focus:ring-lava/20 w-full rounded-lg border
                                    px-3 py-2 text-sm text-white transition-all focus:ring-1
                                    focus:outline-none"
                            />
                        </div>
                    </div>

                    <!-- Career -->
                    <div class="grid grid-cols-2 gap-3">
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Career Start Year
                            </label>
                            <input
                                v-model.number="form.career_start_year"
                                type="number"
                                min="1900"
                                :max="new Date().getFullYear()"
                                class="border-border bg-void/80 focus:border-lava/40
                                    focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm
                                    text-white transition-all focus:ring-1 focus:outline-none"
                            />
                        </div>
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Career End Year
                            </label>
                            <input
                                v-model.number="form.career_end_year"
                                type="number"
                                min="1900"
                                :max="new Date().getFullYear()"
                                class="border-border bg-void/80 focus:border-lava/40
                                    focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm
                                    text-white transition-all focus:ring-1 focus:outline-none"
                            />
                        </div>
                    </div>

                    <!-- Body Modifications -->
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Tattoos
                        </label>
                        <textarea
                            v-model="form.tattoos"
                            rows="2"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none"
                        ></textarea>
                    </div>

                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Piercings
                        </label>
                        <textarea
                            v-model="form.piercings"
                            rows="2"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none"
                        ></textarea>
                    </div>

                    <!-- Toggles -->
                    <div class="flex gap-6">
                        <label class="flex items-center gap-2 text-sm text-white">
                            <input
                                v-model="form.fake_boobs"
                                type="checkbox"
                                class="accent-lava h-4 w-4 rounded"
                            />
                            Enhanced
                        </label>
                        <label class="flex items-center gap-2 text-sm text-white">
                            <input
                                v-model="form.same_sex_only"
                                type="checkbox"
                                class="accent-lava h-4 w-4 rounded"
                            />
                            Same-sex Only
                        </label>
                    </div>

                    <!-- Actions -->
                    <div class="flex justify-end gap-2 pt-2">
                        <button
                            type="button"
                            class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                                hover:text-white"
                            @click="handleClose"
                        >
                            Cancel
                        </button>
                        <button
                            type="submit"
                            :disabled="loading || !form.name"
                            class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs
                                font-semibold text-white transition-all disabled:cursor-not-allowed
                                disabled:opacity-40"
                        >
                            {{
                                loading ? 'Saving...' : isEditMode ? 'Save Changes' : 'Create Actor'
                            }}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    </Teleport>
</template>
