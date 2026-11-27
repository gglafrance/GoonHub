/**
 * Long press on touch screens (the hover actions of desktop): `onLongPress` runs with the
 * pressed element after `delay` ms without moving. Spread the handlers with `v-on`.
 */
export const useLongPress = (onLongPress: (el: HTMLElement) => void, delay = 500) => {
    let timer: ReturnType<typeof setTimeout> | null = null;
    let startX = 0;
    let startY = 0;

    const cancel = () => {
        if (timer) clearTimeout(timer);
        timer = null;
    };

    const touchstart = (e: TouchEvent) => {
        cancel();
        const touch = e.touches[0];
        if (!touch || e.touches.length > 1) return;
        startX = touch.clientX;
        startY = touch.clientY;
        const el = e.currentTarget as HTMLElement;
        timer = setTimeout(() => {
            timer = null;
            onLongPress(el);
        }, delay);
    };

    const touchmove = (e: TouchEvent) => {
        const touch = e.touches[0];
        if (touch && Math.hypot(touch.clientX - startX, touch.clientY - startY) > 10) cancel();
    };

    onScopeDispose(cancel);

    return { touchstart, touchmove, touchend: cancel, touchcancel: cancel };
};
