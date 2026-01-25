export interface Actor {
    id: number;
    uuid: string;
    created_at: string;
    name: string;
    image_url?: string;
    gender?: string;
    birthday?: string;
    date_of_death?: string;
    astrology?: string;
    birthplace?: string;
    ethnicity?: string;
    nationality?: string;
    career_start_year?: number;
    career_end_year?: number;
    height_cm?: number;
    weight_kg?: number;
    measurements?: string;
    cupsize?: string;
    hair_color?: string;
    eye_color?: string;
    tattoos?: string;
    piercings?: string;
    fake_boobs: boolean;
    same_sex_only: boolean;
    video_count?: number;
}

export interface ActorListResponse {
    data: Actor[];
    total: number;
    page: number;
    limit: number;
}

export interface CreateActorInput {
    name: string;
    image_url?: string;
    gender?: string;
    birthday?: string;
    date_of_death?: string;
    astrology?: string;
    birthplace?: string;
    ethnicity?: string;
    nationality?: string;
    career_start_year?: number;
    career_end_year?: number;
    height_cm?: number;
    weight_kg?: number;
    measurements?: string;
    cupsize?: string;
    hair_color?: string;
    eye_color?: string;
    tattoos?: string;
    piercings?: string;
    fake_boobs?: boolean;
    same_sex_only?: boolean;
}

export interface UpdateActorInput {
    name?: string;
    image_url?: string;
    gender?: string;
    birthday?: string;
    date_of_death?: string;
    astrology?: string;
    birthplace?: string;
    ethnicity?: string;
    nationality?: string;
    career_start_year?: number;
    career_end_year?: number;
    height_cm?: number;
    weight_kg?: number;
    measurements?: string;
    cupsize?: string;
    hair_color?: string;
    eye_color?: string;
    tattoos?: string;
    piercings?: string;
    fake_boobs?: boolean;
    same_sex_only?: boolean;
}
