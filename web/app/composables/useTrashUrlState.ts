import type { TrashSort } from '~/types/trash';
import type { TrashFilterState } from '~/utils/trashFilters';

const positiveNumber = (raw: string) => {
    const n = Number(raw);
    return Number.isFinite(n) && n > 0 ? n : undefined;
};
const isoDate = (raw: string) => (/^\d{4}-\d{2}-\d{2}$/.test(raw) ? raw : undefined);

/**
 * State of the trash page kept in the URL (Back, reload, shared links): page, page size, sort,
 * order and every filter. `filters` is a snapshot of the filter refs, `setFilters` replaces them
 * (and goes back to the first page).
 */
export function useTrashUrlState() {
    const page = useUrlPagination();
    const limit = useUrlQueryRef('limit', 20, (raw) => {
        const n = Number(raw);
        return Number.isInteger(n) && n > 0 && n <= 100 ? n : undefined;
    });
    const sort = useUrlQueryRef<TrashSort>('sort', 'trashed_at', (raw) =>
        TRASH_SORTS.some((s) => s.value === raw) ? (raw as TrashSort) : undefined,
    );
    const order = useUrlQueryRef<'asc' | 'desc'>('order', 'desc', (raw) =>
        raw === 'asc' || raw === 'desc' ? raw : undefined,
    );

    const q = useUrlString('q');
    const storage = useUrlQueryRef('storage', '', (raw) =>
        raw === 'none' || /^\d+$/.test(raw) ? raw : undefined,
    );
    const minSizeMb = useUrlQueryRef('min_mb', 0, positiveNumber);
    const maxSizeMb = useUrlQueryRef('max_mb', 0, positiveNumber);
    const minDurationMin = useUrlQueryRef('min_min', 0, positiveNumber);
    const maxDurationMin = useUrlQueryRef('max_min', 0, positiveNumber);
    const from = useUrlQueryRef('from', '', isoDate);
    const to = useUrlQueryRef('to', '', isoDate);
    const resolution = useUrlQueryRef('res', '', (raw) =>
        TRASH_RESOLUTIONS.some((r) => r.value === raw) ? raw : undefined,
    );
    const codec = useUrlString('codec');
    const source = useUrlQueryRef<'' | 'duplicates' | 'other'>('source', '', (raw) =>
        raw === 'duplicates' || raw === 'other' ? raw : undefined,
    );

    const filters = computed<TrashFilterState>(() => ({
        q: q.value,
        storage: storage.value,
        minSizeMb: minSizeMb.value,
        maxSizeMb: maxSizeMb.value,
        minDurationMin: minDurationMin.value,
        maxDurationMin: maxDurationMin.value,
        from: from.value,
        to: to.value,
        resolution: resolution.value,
        codec: codec.value,
        source: source.value,
    }));

    const setFilters = (f: TrashFilterState) => {
        q.value = f.q;
        storage.value = f.storage;
        minSizeMb.value = f.minSizeMb;
        maxSizeMb.value = f.maxSizeMb;
        minDurationMin.value = f.minDurationMin;
        maxDurationMin.value = f.maxDurationMin;
        from.value = f.from;
        to.value = f.to;
        resolution.value = f.resolution;
        codec.value = f.codec;
        source.value = f.source;
        page.value = 1;
    };

    const setSort = (value: TrashSort) => {
        sort.value = value;
        order.value = TRASH_SORTS.find((s) => s.value === value)?.defaultOrder ?? 'desc';
        page.value = 1;
    };

    const toggleOrder = () => {
        order.value = order.value === 'asc' ? 'desc' : 'asc';
        page.value = 1;
    };

    return { page, limit, sort, order, filters, setFilters, setSort, toggleOrder };
}
