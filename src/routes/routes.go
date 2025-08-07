package routes

import (
	"ani4s/src/config"
	files "ani4s/src/modules/files/controllers"
	movies "ani4s/src/modules/movies/controllers"
	"net/http"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")

	// Hello World
	api.GET("hello", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": gin.H{
				"code":    http.StatusOK,
				"message": "Hello world",
			},
		})
	})

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Update when db available
	router.GET("/readyz", func(c *gin.Context) {
		if config.CheckConnection() {
			c.JSON(http.StatusOK, gin.H{"status": "ready"})
		} else {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
		}
	})

	// Newest Movies Routes
	moviesRoutes := api.Group("/phim")
	{
		moviesRoutes.POST("moi-cap-nhat", movies.NewestUpdateMovies)
		moviesRoutes.GET(":slug", movies.GetMovieDetails)
		moviesRoutes.POST("danh-sach", movies.GetMovieList)
		moviesRoutes.POST("tim-kiem", movies.SearchMovies)
		moviesRoutes.POST("the-loai", movies.ListMoviesByCategory)
		moviesRoutes.POST("quoc-gia", movies.ListMoviesByCountry)
		moviesRoutes.GET("categories", movies.ListCategories)
		moviesRoutes.GET("country", movies.ListCountry)
	}

	// Static Proxy MinIO
	staticProxyRoutes := api.Group("/static")
	{
		staticProxyRoutes.GET("/*filepath", files.FileController)
		
	}
}
