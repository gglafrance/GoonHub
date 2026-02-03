package request

type CreateActorRequest struct {
	Name            string   `json:"name" binding:"required"`
	Aliases         []string `json:"aliases"`
	ImageURL        string   `json:"image_url"`
	Gender          string   `json:"gender"`
	Birthday        *string `json:"birthday"`
	DateOfDeath     *string `json:"date_of_death"`
	Astrology       string  `json:"astrology"`
	Birthplace      string  `json:"birthplace"`
	Ethnicity       string  `json:"ethnicity"`
	Nationality     string  `json:"nationality"`
	CareerStartYear *int    `json:"career_start_year"`
	CareerEndYear   *int    `json:"career_end_year"`
	HeightCm        *int    `json:"height_cm"`
	WeightKg        *int    `json:"weight_kg"`
	Measurements    string  `json:"measurements"`
	Cupsize         string  `json:"cupsize"`
	HairColor       string  `json:"hair_color"`
	EyeColor        string  `json:"eye_color"`
	Tattoos         string  `json:"tattoos"`
	Piercings       string  `json:"piercings"`
	FakeBoobs       bool    `json:"fake_boobs"`
	SameSexOnly     bool    `json:"same_sex_only"`
}

type UpdateActorRequest struct {
	Name            *string   `json:"name"`
	Aliases         *[]string `json:"aliases"`
	ImageURL        *string   `json:"image_url"`
	Gender          *string `json:"gender"`
	Birthday        *string `json:"birthday"`
	DateOfDeath     *string `json:"date_of_death"`
	Astrology       *string `json:"astrology"`
	Birthplace      *string `json:"birthplace"`
	Ethnicity       *string `json:"ethnicity"`
	Nationality     *string `json:"nationality"`
	CareerStartYear *int    `json:"career_start_year"`
	CareerEndYear   *int    `json:"career_end_year"`
	HeightCm        *int    `json:"height_cm"`
	WeightKg        *int    `json:"weight_kg"`
	Measurements    *string `json:"measurements"`
	Cupsize         *string `json:"cupsize"`
	HairColor       *string `json:"hair_color"`
	EyeColor        *string `json:"eye_color"`
	Tattoos         *string `json:"tattoos"`
	Piercings       *string `json:"piercings"`
	FakeBoobs       *bool   `json:"fake_boobs"`
	SameSexOnly     *bool   `json:"same_sex_only"`
}

type SetSceneActorsRequest struct {
	ActorIDs []uint `json:"actor_ids"`
}
