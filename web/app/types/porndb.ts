export interface PornDBPerformer {
    id: string;
    slug: string;
    name: string;
    image?: string;
    bio?: string;
}

export interface PornDBPerformerDetails {
    id: string;
    slug: string;
    name: string;
    image?: string;
    bio?: string;
    gender?: string;
    birthday?: string; // ISO date string
    deathday?: string;
    astrology?: string;
    birthplace?: string;
    ethnicity?: string;
    nationality?: string;
    career_start_year?: number;
    career_end_year?: number;
    height?: number; // cm (parsed from "160cm")
    weight?: number; // kg (parsed from "50kg")
    measurements?: string;
    cupsize?: string;
    hair_colour?: string;
    eye_colour?: string;
    tattoos?: string;
    piercings?: string;
    fake_boobs?: boolean;
    same_sex_only?: boolean;
}

export interface PornDBScene {
    id: string;
    title: string;
    description?: string;
    date?: string;
    duration?: number;
    image?: string;
    poster?: string;
    site?: PornDBSite;
    performers?: PornDBScenePerformer[];
    tags?: PornDBTag[];
}

// Lightweight site for scene responses
export interface PornDBSite {
    name: string;
    url?: string;
}

// Full site details for site search/fetch
export interface PornDBSiteDetails {
    id: string;
    uuid?: string;
    slug?: string;
    name: string;
    short_name?: string;
    url?: string;
    description?: string;
    rating?: number;
    logo?: string;
    favicon?: string;
    poster?: string;
    network?: string;
    parent?: string;
    network_id?: string;
    parent_id?: string;
}

export interface PornDBScenePerformer {
    id: string;
    name: string;
    image?: string;
}

export interface PornDBTag {
    id: number;
    name: string;
}
