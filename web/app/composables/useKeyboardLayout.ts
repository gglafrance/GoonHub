import type { KeyboardLayout } from '~/types/settings';

export interface KeyboardKeys {
    frameBack: string;
    frameForward: string;
    speedDecrease: string;
    speedIncreaseShift: string;
    speedDecreaseShift: string;
    speedIncrease: string;
    pagePrev: string;
    pageNext: string;
}

const KEYBOARD_MAPS: Record<KeyboardLayout, KeyboardKeys> = {
    qwerty: {
        frameBack: ',',
        frameForward: '.',
        speedDecrease: '<',
        speedDecreaseShift: ',',
        speedIncrease: '>',
        speedIncreaseShift: '.',
        pagePrev: '[',
        pageNext: ']',
    },
    azerty: {
        frameBack: ';',
        frameForward: ':',
        speedDecrease: '.',
        speedDecreaseShift: ';',
        speedIncrease: '/',
        speedIncreaseShift: ':',
        pagePrev: ')',
        pageNext: '=',
    },
};

// Display-friendly key labels for the shortcuts modal
export interface KeyboardDisplayKeys {
    frameBack: string;
    frameForward: string;
    speedDecrease: string;
    speedIncrease: string;
    pagePrev: string;
    pageNext: string;
}

const DISPLAY_MAPS: Record<KeyboardLayout, KeyboardDisplayKeys> = {
    qwerty: {
        frameBack: ',',
        frameForward: '.',
        speedDecrease: '<',
        speedIncrease: '>',
        pagePrev: '[',
        pageNext: ']',
    },
    azerty: {
        frameBack: ';',
        frameForward: ':',
        speedDecrease: '.',
        speedIncrease: '/',
        pagePrev: ')',
        pageNext: '=',
    },
};

export const useKeyboardLayout = () => {
    const settingsStore = useSettingsStore();

    const layout = computed(() => settingsStore.keyboardLayout);
    const keys = computed(() => KEYBOARD_MAPS[layout.value]);
    const displayKeys = computed(() => DISPLAY_MAPS[layout.value]);

    const setLayout = (newLayout: KeyboardLayout) => {
        settingsStore.setKeyboardLayout(newLayout);
    };

    return {
        layout,
        keys,
        displayKeys,
        setLayout,
    };
};
