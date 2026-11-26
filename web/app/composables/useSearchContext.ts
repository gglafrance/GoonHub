import type { InjectionKey } from 'vue';
import type { SearchStore } from '~/stores/search';

/** The search store instance the search components (components/search) work on. */
export const SEARCH_STORE_KEY: InjectionKey<SearchStore> = Symbol('search-store');

/**
 * Makes the search components below this component use another search store instance (the
 * search picker modal provides its own, isolated from the search page and the URL).
 */
export const provideSearchStore = (store: SearchStore) => provide(SEARCH_STORE_KEY, store);

/** The search store of the context: the provided one, else the search page store. */
export const useSearchContext = (): SearchStore =>
    inject(SEARCH_STORE_KEY, null) ?? useSearchStore();
