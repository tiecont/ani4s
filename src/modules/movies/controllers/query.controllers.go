package movies

import (
	"ani4s/src/modules/movies/lib"
	service "ani4s/src/modules/movies/services"
	"github.com/gin-gonic/gin"
	"net/http"
)

func GetMovieList(c *gin.Context) {
	var req movies.MovieListRequest

	// Bind JSON request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request parameters: " + err.Error(),
		})
		return
	}

	// Set default values
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.SortField == "" {
		req.SortField = "modified.time"
	}
	if req.SortType == "" {
		req.SortType = "desc"
	}
	if req.Limit <= 0 || req.Limit > 64 {
		req.Limit = 20
	}

	// Validate sort_field
	validSortFields := map[string]bool{
		"modified.time": true,
		"_id":           true,
		"year":          true,
	}
	if !validSortFields[req.SortField] {
		req.SortField = "modified.time"
	}

	// Validate sort_type
	if req.SortType != "asc" && req.SortType != "desc" {
		req.SortType = "desc"
	}

	// Validate sort_lang
	if req.SortLang != "" {
		validSortLang := map[string]bool{
			"vietsub":     true,
			"thuyet-minh": true,
			"long-tieng":  true,
		}
		if !validSortLang[req.SortLang] {
			req.SortLang = ""
		}
	}

	// Validate year
	if req.Year != 0 && (req.Year < 1970 || req.Year > 2025) {
		req.Year = 0
	}

	// Call service
	res, err := service.GetMovieList(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to fetch movie list: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, res)
}

func SearchMovies(c *gin.Context) {
	var req movies.MovieSearchRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := service.GetSearchMovies(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
