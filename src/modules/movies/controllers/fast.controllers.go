package movies

import (
	service "ani4s/src/modules/movies/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ListCategories(c *gin.Context) {
	res, err := service.ListAllCategories()
	if err != nil {
		return
	}
	c.JSON(http.StatusOK, res)
}

func ListCountry(c *gin.Context) {
	res, err := service.ListAllCountry()
	if err != nil {
		return
	}
	c.JSON(http.StatusOK, res)
}
