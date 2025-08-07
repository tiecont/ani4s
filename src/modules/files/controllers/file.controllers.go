package files

import (
	file "ani4s/src/modules/files/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

func FileController(c *gin.Context) {
	filepath := c.Param("filepath")
	if filepath == "" || filepath == "/" {
		c.JSON(http.StatusBadRequest, gin.H{"e": "invalid filepath"})
		return
	}

	reader, size, contentType, e := file.FileService(filepath)
	if e != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": e.Error()})
		return
	}

	c.DataFromReader(http.StatusOK, size, contentType, reader, nil)
}
