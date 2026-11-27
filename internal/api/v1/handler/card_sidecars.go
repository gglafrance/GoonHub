package handler

import (
	"goonhub/internal/api/v1/response"
	"goonhub/internal/data"
)

// What the scene lists add to their items so the scene cards can show the user's card
// template: tag and actor names (join tables), and the user's own interactions as
// "sidecar" maps keyed by scene ID. Shared by the scene list, the related scenes and the
// explorer, so a card looks the same on every page.

// fillCardTagsActors sets the tag and actor names of the items when the card fields ask
// for them. A failed lookup leaves the names out (the card simply doesn't show them).
func fillCardTagsActors(tagRepo data.TagRepository, actorRepo data.ActorRepository, items []response.SceneListItem, sceneIDs []uint, fields response.CardFields) {
	if len(sceneIDs) == 0 {
		return
	}
	if fields.Tags && tagRepo != nil {
		if tagsByScene, err := tagRepo.GetSceneTagsMultiple(sceneIDs); err == nil {
			for i := range items {
				if tags, ok := tagsByScene[items[i].ID]; ok {
					names := make([]string, len(tags))
					for j, t := range tags {
						names[j] = t.Name
					}
					items[i].Tags = names
				}
			}
		}
	}
	if fields.Actors && actorRepo != nil {
		if actorsByScene, err := actorRepo.GetSceneActorsMultiple(sceneIDs); err == nil {
			for i := range items {
				if actors, ok := actorsByScene[items[i].ID]; ok {
					names := make([]string, len(actors))
					for j, a := range actors {
						names[j] = a.Name
					}
					items[i].Actors = names
				}
			}
		}
	}
}

// loadCardSidecars loads the user's rating, like, jizz count and watched maps the card
// fields ask for (nothing for an anonymous request). A failed lookup leaves that map out.
func loadCardSidecars(repo data.InteractionRepository, userID uint, sceneIDs []uint, fields response.CardFields) response.CardSidecars {
	var out response.CardSidecars
	if repo == nil || userID == 0 {
		return out
	}
	if fields.Rating {
		if ratings, err := repo.GetRatingsBySceneIDs(userID, sceneIDs); err == nil {
			out.Ratings = ratings
		}
	}
	if fields.Liked {
		if likes, err := repo.GetLikesBySceneIDs(userID, sceneIDs); err == nil {
			out.Likes = likes
		}
	}
	if fields.JizzCount {
		if jizzCounts, err := repo.GetJizzCountsBySceneIDs(userID, sceneIDs); err == nil {
			out.JizzCounts = jizzCounts
		}
	}
	if fields.Watched {
		if watched, err := repo.GetWatchedBySceneIDs(userID, sceneIDs); err == nil {
			out.Watched = watched
		}
	}
	return out
}

// addCardSidecars puts the loaded maps into a gin.H style response.
func addCardSidecars(resp map[string]any, s response.CardSidecars) {
	if s.Ratings != nil {
		resp["ratings"] = s.Ratings
	}
	if s.Likes != nil {
		resp["likes"] = s.Likes
	}
	if s.JizzCounts != nil {
		resp["jizz_counts"] = s.JizzCounts
	}
	if s.Watched != nil {
		resp["watched"] = s.Watched
	}
}
