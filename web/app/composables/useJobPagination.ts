/**
 * Composable for job history pagination with localStorage persistence.
 */
export const useJobPagination = (options: { onPageChange: () => void }) => {
    const pageSizes = [10, 25, 50] as const;
    const page = ref(1);
    const limit = ref(Number(localStorage.getItem('jobs-page-size')) || 10);
    const total = ref(0);

    const totalPages = computed(() => Math.ceil(total.value / limit.value));

    const prevPage = () => {
        if (page.value > 1) {
            page.value--;
            options.onPageChange();
        }
    };

    const nextPage = () => {
        if (page.value < totalPages.value) {
            page.value++;
            options.onPageChange();
        }
    };

    const changePageSize = (size: number) => {
        limit.value = size;
        page.value = 1;
        localStorage.setItem('jobs-page-size', String(size));
        options.onPageChange();
    };

    const setTotal = (value: number) => {
        total.value = value;
    };

    return {
        pageSizes,
        page,
        limit,
        total,
        totalPages,
        prevPage,
        nextPage,
        changePageSize,
        setTotal,
    };
};
