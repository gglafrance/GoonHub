export interface ScanHistory {
    id: number;
    status: 'running' | 'completed' | 'failed' | 'cancelled';
    started_at: string;
    completed_at?: string;
    paths_scanned: number;
    files_found: number;
    videos_added: number;
    videos_skipped: number;
    errors: number;
    error_message?: string;
    current_path?: string;
    current_file?: string;
    created_at: string;
}

export interface ScanStatus {
    running: boolean;
    current_scan?: ScanHistory;
}

export interface ScanHistoryResponse {
    data: ScanHistory[];
    total: number;
    page: number;
    limit: number;
}

export interface ScanProgressEvent {
    files_found: number;
    videos_added: number;
    videos_skipped: number;
    errors: number;
    current_path?: string;
    current_file?: string;
}

export interface ScanVideoAddedEvent {
    video_id: number;
    video_path: string;
    title: string;
}
