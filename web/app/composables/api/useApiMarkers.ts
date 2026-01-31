/**
 * Marker-related API operations: CRUD for video markers.
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
    MarkerWithVideo,
    MarkersResponse,
    LabelSuggestionsResponse,
    PaginatedResponse,
} from '~/types/marker';

export const useApiMarkers = () => {
    const { fetchOptions, getAuthHeaders, handleResponse, handleResponseWithNoContent } = useApiCore();

    const fetchMarkers = async (videoId: number): Promise<MarkersResponse> => {
        const response = await fetch(`/api/v1/videos/${videoId}/markers`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const createMarker = async (videoId: number, data: CreateMarkerRequest): Promise<Marker> => {
        const response = await fetch(`/api/v1/videos/${videoId}/markers`, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify(data),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updateMarker = async (
        videoId: number,
        markerId: number,
        data: UpdateMarkerRequest
    ): Promise<Marker> => {
        const response = await fetch(`/api/v1/videos/${videoId}/markers/${markerId}`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(data),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const deleteMarker = async (videoId: number, markerId: number): Promise<void> => {
        const response = await fetch(`/api/v1/videos/${videoId}/markers/${markerId}`, {
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
        sort: string = 'count_desc'
    ): Promise<PaginatedResponse<MarkerLabelGroup>> => {
        const params = new URLSearchParams({
            page: String(page),
            limit: String(limit),
            sort,
        });
        const response = await fetch(`/api/v1/markers?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchMarkersByLabel = async (
        label: string,
        page: number = 1,
        limit: number = 20
    ): Promise<PaginatedResponse<MarkerWithVideo>> => {
        const params = new URLSearchParams({
            label,
            page: String(page),
            limit: String(limit),
        });
        const response = await fetch(`/api/v1/markers/by-label?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    return {
        fetchMarkers,
        createMarker,
        updateMarker,
        deleteMarker,
        fetchLabelSuggestions,
        fetchLabelGroups,
        fetchMarkersByLabel,
    };
};
