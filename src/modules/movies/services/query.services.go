package movies

import (
	"ani4s/src/config"
	movies "ani4s/src/modules/movies/lib"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

func GetMovieList(req movies.MovieListRequest) (map[string]interface{}, error) {
	rdb := config.RDB
	ctx := config.Ctx

	targetURL := buildListURL(req)
	cacheKey := buildCacheKey(req)

	// 1. Try Redis cache first
	cached, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		var cachedData map[string]interface{}
		if err := json.Unmarshal([]byte(cached), &cachedData); err == nil {
			cachedData["from_cache"] = true
			return cachedData, nil
		}
	}

	// 2. Fetch from API if no cache
	responseBody, err := MakeAnonymousRequest(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data from API: %w", err)
	}

	var apiResponse map[string]interface{}
	if err := json.Unmarshal(responseBody, &apiResponse); err != nil {
		return map[string]interface{}{
			"success":  false,
			"error":    "Failed to parse API response",
			"raw_data": string(responseBody),
		}, nil
	}

	apiResponse["from_cache"] = false
	apiResponse["request_url"] = targetURL
	apiResponse["timestamp"] = time.Now().Unix()

	// 3. Save raw response in Redis (short TTL if recently updated list)
	var ttl time.Duration
	ttl = 16 * time.Hour

	jsonStr, _ := json.Marshal(apiResponse)
	_ = rdb.Set(ctx, cacheKey, jsonStr, ttl).Err()

	// 4. Save key for later background indexing
	tagKey := "movie_list:cached_keys"
	rdb.SAdd(ctx, tagKey, cacheKey)
	rdb.Expire(ctx, tagKey, 12*time.Hour)

	// Trả lại response gốc dạng map
	var returnMap map[string]interface{}
	_ = json.Unmarshal(jsonStr, &returnMap)

	return returnMap, nil
}

// GetSearchMovies performs a search query using phimapi.com and caches the response
func GetSearchMovies(req movies.MovieSearchRequest) (map[string]interface{}, error) {
	rdb := config.RDB
	ctx := config.Ctx

	targetURL := buildSearchURL(req)
	cacheKey := buildSearchCacheKey(req)

	// 1. Try Redis cache first
	cached, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		var cachedData map[string]interface{}
		if err := json.Unmarshal([]byte(cached), &cachedData); err == nil {
			cachedData["from_cache"] = true
			return cachedData, nil
		}
	}

	// 2. Call API
	responseBody, err := MakeAnonymousRequest(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch search results from API: %w", err)
	}

	var apiResponse map[string]interface{}
	if err := json.Unmarshal(responseBody, &apiResponse); err != nil {
		return map[string]interface{}{
			"success":  false,
			"error":    "Failed to parse API response",
			"raw_data": string(responseBody),
		}, nil
	}

	apiResponse["from_cache"] = false
	apiResponse["request_url"] = targetURL
	apiResponse["timestamp"] = time.Now().Unix()

	// 3. Cache TTL for search is shorter (because it changes more)
	jsonStr, _ := json.Marshal(apiResponse)
	_ = rdb.Set(ctx, cacheKey, jsonStr, 16*time.Hour).Err()

	// 4. Track cached key for potential indexing
	tagKey := "movie_search:cached_keys"
	rdb.SAdd(ctx, tagKey, cacheKey)
	rdb.Expire(ctx, tagKey, 12*time.Hour)

	var returnMap map[string]interface{}
	_ = json.Unmarshal(jsonStr, &returnMap)
	return returnMap, nil
}

// buildListURL constructs the target API URL with query params
func buildListURL(req movies.MovieListRequest) string {
	baseURL := fmt.Sprintf("https://phimapi.com/v1/api/danh-sach/%s", req.TypeList)
	params := url.Values{}
	params.Add("page", fmt.Sprintf("%d", req.Page))
	params.Add("sort_field", req.SortField)
	params.Add("sort_type", req.SortType)
	params.Add("limit", fmt.Sprintf("%d", req.Limit))
	if req.SortLang != "" {
		params.Add("sort_lang", req.SortLang)
	}
	if req.Category != "" {
		params.Add("category", req.Category)
	}
	if req.Country != "" {
		params.Add("country", req.Country)
	}
	if req.Year != 0 {
		params.Add("year", fmt.Sprintf("%d", req.Year))
	}
	return baseURL + "?" + params.Encode()
}

// buildCacheKey creates a unique Redis cache key for a list query
func buildCacheKey(req movies.MovieListRequest) string {
	return fmt.Sprintf("movie_list:%s:%d:%s:%s:%s:%s:%s:%d:%d",
		req.TypeList, req.Page, req.SortField, req.SortType,
		req.SortLang, req.Category, req.Country, req.Year, req.Limit)
}

func buildSearchURL(req movies.MovieSearchRequest) string {
	baseURL := "https://phimapi.com/v1/api/tim-kiem"
	params := url.Values{}
	params.Add("keyword", req.Keyword)
	params.Add("page", fmt.Sprintf("%d", req.Page))
	params.Add("sort_field", req.SortField)
	params.Add("sort_type", req.SortType)
	params.Add("limit", fmt.Sprintf("%d", req.Limit))

	if req.SortLang != "" {
		params.Add("sort_lang", req.SortLang)
	}
	if req.Category != "" {
		params.Add("category", req.Category)
	}
	if req.Country != "" {
		params.Add("country", req.Country)
	}
	if req.Year != 0 {
		params.Add("year", fmt.Sprintf("%d", req.Year))
	}

	return baseURL + "?" + params.Encode()
}

func buildSearchCacheKey(req movies.MovieSearchRequest) string {
	return fmt.Sprintf("movie_search:%s:%d:%s:%s:%s:%s:%s:%d:%d",
		req.Keyword, req.Page, req.SortField, req.SortType,
		req.SortLang, req.Category, req.Country, req.Year, req.Limit)
}
