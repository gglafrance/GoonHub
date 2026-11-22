/**
 * Marker-related API operations: CRUD for scene markers.
 *
 * Response shape conventions:
 * - Non-paginated lists: { markers: Marker[] } or { labels: MarkerLabelSuggestion[] }
 * - Paginated lists: { data: T[], pagination: { page, limit, total_items, total_pages } }
 */
import type {
    Marker,
    CreateMarkerRequest,
    UpdateMarkerRequest,
    MarkerLabelGroup,
    MarkerWithScene,
    MarkerTagInfo,
    MarkersResponse,
    LabelSuggestionsResponse,
    LabelTagsResponse,
    MarkerTagsResponse,
    PaginatedResponse,
    SavedMarkerFilter,
    SavedMarkerFilterValues,
    MarkerDedupResult,
    MarkerDedupScope,
} from '~/types/marker';
import type { Tag } from '~/types/tag';

export const useApiMarkers = () => {
    const { fetchOptions, getAuthHeaders, handleResponse, handleResponseWithNoContent } =
        useApiCore();

    const fetchMarkers = async (sceneId: number): Promise<MarkersResponse> => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/markers`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const createMarker = async (sceneId: number, data: CreateMarkerRequest): Promise<Marker> => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/markers`, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify(data),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updateMarker = async (
        sceneId: number,
        markerId: number,
        data: UpdateMarkerRequest,
    ): Promise<Marker> => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/markers/${markerId}`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(data),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const deleteMarker = async (sceneId: number, markerId: number): Promise<void> => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/markers/${markerId}`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponseWithNoContent(response);
    };

    const fetchLabelSuggestions = async (): Promise<LabelSuggestionsResponse> => {
        const response = await fetch('/api/v1/markers/labels', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchLabelGroups = async (
        page: number = 1,
        limit: number = 20,
        sort: string = 'count_desc',
        extra: Record<string, string> = {},
    ): Promise<PaginatedResponse<MarkerLabelGroup>> => {
        const params = new URLSearchParams({
            page: String(page),
            limit: String(limit),
            sort,
            ...extra,
        });
        const response = await fetch(`/api/v1/markers?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchAllMarkers = async (
        page: number = 1,
        limit: number = 20,
        sort: string = 'label_asc',
        extra: Record<string, string> = {},
    ): Promise<PaginatedResponse<MarkerWithScene>> => {
        const params = new URLSearchParams({
            page: String(page),
            limit: String(limit),
            sort,
            ...extra,
        });
        const response = await fetch(`/api/v1/markers/all?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchMarkersByLabel = async (
        label: string,
        page: number = 1,
        limit: number = 20,
        extra: Record<string, string> = {},
    ): Promise<PaginatedResponse<MarkerWithScene>> => {
        const params = new URLSearchParams({
            label,
            page: String(page),
            limit: String(limit),
            ...extra,
        });
        const response = await fetch(`/api/v1/markers/by-label?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    // Label tag methods
    const fetchLabelTags = async (label: string): Promise<Tag[]> => {
        const params = new URLSearchParams({ label });
        const response = await fetch(`/api/v1/markers/label-tags?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        const data: LabelTagsResponse = await handleResponse(response);
        return data.tags || [];
    };

    const setLabelTags = async (label: string, tagIds: number[]): Promise<Tag[]> => {
        const params = new URLSearchParams({ label });
        const response = await fetch(`/api/v1/markers/label-tags?${params}`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ tag_ids: tagIds }),
            ...fetchOptions(),
        });
        const data: LabelTagsResponse = await handleResponse(response);
        return data.tags || [];
    };

    // Individual marker tag methods
    const fetchMarkerTags = async (markerId: number): Promise<MarkerTagInfo[]> => {
        const response = await fetch(`/api/v1/markers/${markerId}/tags`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        const data: MarkerTagsResponse = await handleResponse(response);
        return data.tags || [];
    };

    const setMarkerTags = async (markerId: number, tagIds: number[]): Promise<MarkerTagInfo[]> => {
        const response = await fetch(`/api/v1/markers/${markerId}/tags`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ tag_ids: tagIds }),
            ...fetchOptions(),
        });
        const data: MarkerTagsResponse = await handleResponse(response);
        return data.tags || [];
    };

    const addMarkerTags = async (markerId: number, tagIds: number[]): Promise<MarkerTagInfo[]> => {
        const response = await fetch(`/api/v1/markers/${markerId}/tags`, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ tag_ids: tagIds }),
            ...fetchOptions(),
        });
        const data: MarkerTagsResponse = await handleResponse(response);
        return data.tags || [];
    };

    // Saved marker filters (advanced marker filters presets)
    const fetchSavedMarkerFilters = async (): Promise<{ filters: SavedMarkerFilter[] }> => {
        const response = await fetch('/api/v1/marker-filters', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const createSavedMarkerFilter = async (
        name: string,
        filters: SavedMarkerFilterValues,
    ): Promise<SavedMarkerFilter> => {
        const response = await fetch('/api/v1/marker-filters', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ name, filters }),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updateSavedMarkerFilter = async (
        uuid: string,
        data: { name?: string; filters?: SavedMarkerFilterValues },
    ): Promise<SavedMarkerFilter> => {
        const response = await fetch(`/api/v1/marker-filters/${uuid}`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(data),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const deleteSavedMarkerFilter = async (uuid: string): Promise<void> => {
        const response = await fetch(`/api/v1/marker-filters/${uuid}`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponseWithNoContent(response);
    };

    // Admin: folds duplicate markers (dry run unless dryRun is false)
    const deduplicateMarkers = async (
        scope: MarkerDedupScope,
        dryRun: boolean,
    ): Promise<MarkerDedupResult> => {
        const response = await fetch('/api/v1/admin/markers/deduplicate', {
            ...fetchOptions(),
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ ...scope, dry_run: dryRun }),
        });
        return handleResponse(response);
    };

    return {
        deduplicateMarkers,
        fetchSavedMarkerFilters,
        createSavedMarkerFilter,
        updateSavedMarkerFilter,
        deleteSavedMarkerFilter,
        fetchMarkers,
        createMarker,
        updateMarker,
        deleteMarker,
        fetchLabelSuggestions,
        fetchLabelGroups,
        fetchAllMarkers,
        fetchMarkersByLabel,
        // Label tag methods
        fetchLabelTags,
        setLabelTags,
        // Marker tag methods
        fetchMarkerTags,
        setMarkerTags,
        addMarkerTags,
    };
};
