package movies

import (
	lib "ani4s/src/modules/movies/lib"
	movies "ani4s/src/modules/movies/services"
	"github.com/gin-gonic/gin"
	"net/http"
)

func NewestUpdateMovies(c *gin.Context) {
	var req struct {
		Page int `json:"page"`
		V    int `json:"v"`
	}
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := movies.GetListNewestMovies(req.Page, req.V)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func GetMovieDetails(c *gin.Context) {
	slug := c.Param("slug")

	res, err := movies.GetDetailsMovie(slug)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func ListMoviesByCategory(c *gin.Context) {
	var req lib.MoviesByCategoryRequest
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}

	res, err := movies.ListMoviesByCategory(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func ListMoviesByCountry(c *gin.Context) {
	var req lib.MoviesByCategoryRequest
	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}

	res, err := movies.ListMoviesByCountry(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}
