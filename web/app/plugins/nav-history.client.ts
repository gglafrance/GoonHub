/** Records each history entry of the tab, to find the list a scene was opened from. */
export default defineNuxtPlugin(() => {
    const router = useRouter();
    router.afterEach((to, _from, failure) => {
        if (failure) return;
        recordCurrentNavEntry(to.fullPath);
    });
    // The initial navigation may be over before this plugin runs
    router.isReady().then(() => recordCurrentNavEntry(router.currentRoute.value.fullPath));
});
