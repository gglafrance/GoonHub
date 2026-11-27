/**
 * Copy to the clipboard with feedback, for every "copy" button of the app (copyText in
 * utils/clipboard.ts: Clipboard API, else the execCommand fallback for plain http).
 * `copied` is true for a moment after a copy; when copying is impossible, `manualText` holds
 * the text so the component can show it selected (UiCopyFallback) to be copied by hand.
 */
export const useClipboard = (resetMs = 2000) => {
    const copied = ref(false);
    const manualText = ref<string | null>(null);
    let timer: ReturnType<typeof setTimeout> | null = null;

    const copy = async (text: string): Promise<boolean> => {
        const ok = await copyText(text);
        if (timer) clearTimeout(timer);
        timer = null;
        copied.value = ok;
        manualText.value = ok ? null : text;
        if (ok) {
            timer = setTimeout(() => {
                copied.value = false;
                timer = null;
            }, resetMs);
        }
        return ok;
    };

    const dismiss = () => {
        manualText.value = null;
    };

    onScopeDispose(() => {
        if (timer) clearTimeout(timer);
    });

    return { copied, manualText, copy, dismiss };
};
