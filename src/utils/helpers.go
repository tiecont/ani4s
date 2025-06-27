package utils

import (
	"github.com/gin-gonic/gin"
	"math"
	"net/http"
	"strconv"
)

func Paginate(total int64, page, perPage int) (map[string]interface{}, error) {
	// Avoid division by zero
	if perPage <= 0 {
		perPage = 1
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))

	// Determine next and previous pages
	var nextPage, prevPage *int
	if page < totalPages {
		next := page + 1
		nextPage = &next
	}
	if page > 1 {
		prev := page - 1
		prevPage = &prev
	}

	// Construct and return pagination data
	return map[string]interface{}{
		"current_page":   page,
		"items_per_page": perPage,
		"next_page":      nextPage,
		"previous_page":  prevPage,
		"total_count":    total,
		"total_pages":    totalPages,
	}, nil
}

// ConvertStringToInt64 is the function will receive a string and return int64
func ConvertStringToInt64(str string) int64 {
	i, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		panic(err)
	}
	return i
}

// ConvertInt64ToString is the function will receive an int64 and return string
func ConvertInt64ToString(i int64) string {
	return strconv.FormatInt(i, 10)
}

// ConvertInt64ToUint is the function will receive an int64 and return uint format
func ConvertInt64ToUint(i int64) uint {
	return uint(i)
}

// ConvertStringToUint is the function to convert string to uint format
func ConvertStringToUint(str string) uint {
	i, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		panic(err)
	}
	return uint(i)
}

// ServiceError to define return exception for system
type ServiceError struct {
	StatusCode int
	Message    string
}

func (e *ServiceError) Error() string {
	return e.Message
}

// CalculateOffsetStruct is the struct to define return result for calculate service
type CalculateOffsetStruct struct {
	CurrentPage  int
	ItemsPerPage int
	OrderBy      string
	SortBy       string
	Offset       int
}

// CalculateOffset is the function to calculate offset for list service
func CalculateOffset(currentPage, itemsPerPage int, sortBy, orderBy string) CalculateOffsetStruct {
	if orderBy == "" {
		orderBy = "created_at"
	}
	if sortBy != "asc" && sortBy != "desc" {
		sortBy = "desc"
	}

	// Calculate offset for pagination
	offset := (currentPage - 1) * itemsPerPage
	if offset < 0 {
		offset = 0
	}

	return CalculateOffsetStruct{
		CurrentPage:  currentPage,
		ItemsPerPage: itemsPerPage,
		OrderBy:      orderBy,
		SortBy:       sortBy,
		Offset:       offset,
	}
}

// BindJson is a function to bind the json request
func BindJson(c *gin.Context, request interface{}) *ServiceError {
	if err := c.ShouldBind(&request); err != nil {
		return &ServiceError{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid input",
		}
	}
	return nil
}
