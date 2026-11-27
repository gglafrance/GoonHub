<script setup lang="ts">
import type { NavItem } from '~/utils/navItems';

// "Manage" dropdown at the end of the nav row: the curation/management pages. Opens on click
// (touch too), under the button and extending to its right; closes on outside click, Esc,
// Tab out, item click and route change. Arrow keys, Home and End move between the items.
const props = defineProps<{ items: NavItem[] }>();

const route = useRoute();
const open = ref(false);
const buttonRef = ref<HTMLButtonElement | null>(null);
const panelRef = ref<HTMLDivElement | null>(null);
const position = ref({ top: 0, left: 0 });
const menuId = useId();

const PANEL_WIDTH = 192;

const active = computed(() => isAnyNavItemActive(route.path, props.items));

function itemEls(): HTMLElement[] {
    return Array.from(panelRef.value?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? []);
}

function focusItem(index: number) {
    itemEls()[index]?.focus();
}

function updatePosition() {
    if (!buttonRef.value) return;
    const rect = buttonRef.value.getBoundingClientRect();
    position.value = {
        top: rect.bottom + 6,
        left: menuPanelLeft(rect.left, PANEL_WIDTH, window.innerWidth),
    };
}

async function show(focus: 'first' | 'last' | null = null) {
    updatePosition();
    open.value = true;
    if (focus) {
        await nextTick();
        focusItem(focus === 'first' ? 0 : props.items.length - 1);
    }
}

function close(returnFocus = false) {
    if (!open.value) return;
    open.value = false;
    if (returnFocus) buttonRef.value?.focus();
}

function onButtonClick(e: MouseEvent) {
    if (open.value) {
        close();
        return;
    }
    // detail 0: activated with the keyboard (Enter/Space), focus goes into the menu
    show(e.detail === 0 ? 'first' : null);
}

function onButtonKeydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault();
        show(e.key === 'ArrowDown' ? 'first' : 'last');
    }
}

function onPanelKeydown(e: KeyboardEvent) {
    if (e.key === 'Tab') {
        // the panel lives in <body>: hand the focus back to the button, the Tab goes on from it
        close(true);
        return;
    }
    const els = itemEls();
    const current = els.indexOf(document.activeElement as HTMLElement);
    const next = menuFocusIndex(current, els.length, e.key);
    if (next !== null) {
        e.preventDefault();
        els[next]?.focus();
    }
}

function onPointerDownOutside(e: PointerEvent) {
    const target = e.target as Node;
    if (panelRef.value?.contains(target) || buttonRef.value?.contains(target)) return;
    close();
}

function onDocumentKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
        e.stopPropagation();
        close(true);
    }
}

function teardown() {
    window.removeEventListener('resize', updatePosition);
    document.removeEventListener('pointerdown', onPointerDownOutside, true);
    document.removeEventListener('keydown', onDocumentKeydown, true);
}

watch(open, (isOpen) => {
    if (isOpen) {
        window.addEventListener('resize', updatePosition);
        document.addEventListener('pointerdown', onPointerDownOutside, true);
        document.addEventListener('keydown', onDocumentKeydown, true);
    } else {
        teardown();
    }
});

watch(
    () => route.fullPath,
    () => close(),
);

onBeforeUnmount(teardown);
</script>

<template>
    <div class="relative">
        <button
            ref="buttonRef"
            type="button"
            class="text-dim flex items-center justify-center gap-0.5 rounded-md px-2 py-1.5
                text-[11px] font-medium transition-all hover:bg-white/5 hover:text-white
                sm:justify-start sm:gap-1.5 sm:px-2.5 sm:py-1"
            :class="{ '!text-lava bg-lava/10': active, 'bg-white/5 text-white': open && !active }"
            title="Manage"
            aria-haspopup="menu"
            :aria-expanded="open"
            :aria-controls="open ? menuId : undefined"
            @click="onButtonClick"
            @keydown="onButtonKeydown"
        >
            <Icon name="heroicons:wrench-screwdriver" size="18" class="sm:!h-3.5 sm:!w-3.5" />
            <span class="hidden sm:inline">Manage</span>
            <Icon
                name="heroicons:chevron-down"
                size="12"
                class="transition-transform duration-150"
                :class="{ 'rotate-180': open }"
            />
        </button>

        <Teleport to="body">
            <Transition
                enter-active-class="transition duration-100 ease-out"
                enter-from-class="-translate-y-1 opacity-0"
                leave-active-class="transition duration-75 ease-in"
                leave-to-class="-translate-y-1 opacity-0"
            >
                <div
                    v-if="open"
                    :id="menuId"
                    ref="panelRef"
                    role="menu"
                    aria-label="Manage"
                    class="border-border bg-panel/95 fixed z-[60] rounded-lg border p-1 shadow-2xl
                        backdrop-blur-md"
                    :style="{
                        top: `${position.top}px`,
                        left: `${position.left}px`,
                        width: `${PANEL_WIDTH}px`,
                    }"
                    @keydown="onPanelKeydown"
                >
                    <div
                        class="px-2.5 pt-1.5 pb-1 text-[10px] font-medium tracking-wider
                            text-white/40 uppercase"
                    >
                        Manage
                    </div>
                    <NuxtLink
                        v-for="item in items"
                        :key="item.to"
                        :to="item.to"
                        role="menuitem"
                        class="text-dim focus-visible:ring-lava/40 flex items-center gap-2
                            rounded-md px-2.5 py-1.5 text-[11px] font-medium transition-colors
                            outline-none hover:bg-white/5 hover:text-white focus-visible:bg-white/5
                            focus-visible:text-white focus-visible:ring-1"
                        :class="{
                            '!text-lava bg-lava/10': isNavItemActive(route.path, item.to),
                        }"
                        @click="close()"
                    >
                        <Icon :name="item.icon" size="14" class="shrink-0" />
                        {{ item.label }}
                    </NuxtLink>
                </div>
            </Transition>
        </Teleport>
    </div>
</template>
