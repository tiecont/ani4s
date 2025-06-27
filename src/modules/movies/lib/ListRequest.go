package movies

type MovieListRequest struct {
	TypeList  string `json:"type_list" binding:"required"`
	Page      int    `json:"page"`
	SortField string `json:"sort_field"`
	SortType  string `json:"sort_type"`
	SortLang  string `json:"sort_lang"`
	Category  string `json:"category"`
	Country   string `json:"country"`
	Year      int    `json:"year"`
	Limit     int    `json:"limit"`
}

type MovieSearchRequest struct {
	Keyword   string `json:"keyword"`
	Page      int    `json:"page"`
	SortField string `json:"sort_field"`
	SortType  string `json:"sort_type"`
	SortLang  string `json:"sort_lang"`
	Category  string `json:"category"`
	Country   string `json:"country"`
	Year      int    `json:"year"`
	Limit     int    `json:"limit"`
}

type MoviesByCategoryRequest struct {
	Category  string `json:"category"`
	Page      int    `json:"page"`
	SortField string `json:"sort_field"`
	SortType  string `json:"sort_type"`
	SortLang  string `json:"sort_lang"`
	Country   string `json:"country"`
	Year      int    `json:"year"`
	Limit     int    `json:"limit"`
}
