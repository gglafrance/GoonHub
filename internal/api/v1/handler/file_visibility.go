package handler

import (
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"

	"github.com/gin-gonic/gin"
)

// fileVisibility returns the file information the authenticated user may see.
func fileVisibility(c *gin.Context) response.FileVisibility {
	return response.FileVisibilityFor(middleware.GetAccess(c))
}

// isSizeSort reports whether a sort order sorts by file size.
func isSizeSort(sort string) bool {
	return sort == "size_asc" || sort == "size_desc"
}

// refuseHiddenSizeQuery answers 403 and returns true when a size filter or a size sort is
// used by a user without files:view_size.
func refuseHiddenSizeQuery(c *gin.Context, vis response.FileVisibility, sort string, sizeFiltered bool) bool {
	if vis.Size || (!sizeFiltered && !isSizeSort(sort)) {
		return false
	}
	response.Error(c, apperrors.NewForbiddenError("file size filters and sorts need the files:view_size permission"))
	return true
}
