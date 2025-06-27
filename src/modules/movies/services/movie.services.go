package movies

import (
	"ani4s/src/config"
	lib "ani4s/src/modules/movies/lib"
	movies "ani4s/src/modules/movies/models"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MakeAnonymousRequest makes a GET request while masking server information
func MakeAnonymousRequest(url string) ([]byte, error) {
	// Create a custom HTTP client with modifications to hide server info
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
			DisableKeepAlives: true, // Prevent connection reuse
		},
	}

	// Create the request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add headers to mask the request origin and mimic a regular browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	// Remove or mask headers that might reveal server information
	req.Header.Del("X-Forwarded-For")
	req.Header.Del("X-Real-IP")
	req.Header.Del("X-Forwarded-Proto")
	req.Header.Del("X-Forwarded-Host")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Handle compressed responses
	var reader io.Reader = resp.Body

	// Check if response is gzip compressed
	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	// Read the response body
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func GetListNewestMovies(page, v int) (map[string]interface{}, error) {
	rdb := config.RDB
	ctx := config.Ctx

	// Define URL
	var targetURL string
	if v == 1 {
		targetURL = fmt.Sprintf("https://phimapi.com/danh-sach/phim-moi-cap-nhat?page=%d", page)
	} else {
		targetURL = fmt.Sprintf("https://phimapi.com/danh-sach/phim-moi-cap-nhat-v%d?page=%d", v, page)
	}

	cacheKey := fmt.Sprintf("newest_movies:%s", targetURL)

	// Try Redis
	cached, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		var cachedData map[string]interface{}
		if err := json.Unmarshal([]byte(cached), &cachedData); err == nil {
			return cachedData, nil
		}
	}

	// Request API
	responseBody, err := MakeAnonymousRequest(targetURL)
	if err != nil {
		return nil, err
	}

	// Parse response JSON
	var parsed struct {
		Items []movies.Movie `json:"items"`
	}
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return map[string]interface{}{"data": string(responseBody)}, nil
	}

	// Cache response (TTL 30 phút)
	jsonStr, _ := json.Marshal(parsed)
	_ = rdb.Set(ctx, cacheKey, jsonStr, 30*time.Minute).Err()

	// Trả lại response gốc dạng map
	var returnMap map[string]interface{}
	_ = json.Unmarshal(jsonStr, &returnMap)

	return returnMap, nil
}

func GetDetailsMovie(slug string) (map[string]interface{}, error) {
	rdb := config.RDB
	ctx := config.Ctx
	db := config.DB

	targetURL := fmt.Sprintf("https://phimapi.com/phim/%s", slug)
	cacheKey := fmt.Sprintf("details_movies:%s", targetURL)

	// 1. Try cache
	if cached, err := rdb.Get(ctx, cacheKey).Result(); err == nil {
		var cachedData map[string]interface{}
		if jsonErr := json.Unmarshal([]byte(cached), &cachedData); jsonErr == nil {
			return cachedData, nil
		}
	}

	// 2. Try from DB
	if res, err := GetMovieDetailsFromDB(slug); err == nil {
		return res, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// 3. Call external API
	responseBody, err := MakeAnonymousRequest(targetURL)
	if err != nil {
		return nil, err
	}

	// 4. Parse API response
	var data movies.MovieDetails
	if err := json.Unmarshal(responseBody, &data); err != nil {
		return map[string]interface{}{"data": string(responseBody)}, nil
	}

	var categories []movies.Category
	for _, cat := range data.Movie.Categories {
		var c movies.Category
		if err := db.FirstOrCreate(&c, movies.Category{ID: cat.ID}).Error; err == nil {
			categories = append(categories, c)
		}
	}
	data.Movie.Categories = categories

	var countries []movies.Country
	for _, country := range data.Movie.Countries {
		var c movies.Country
		if err := db.FirstOrCreate(&c, movies.Country{ID: country.ID}).Error; err == nil {
			countries = append(countries, c)
		}
	}
	data.Movie.Countries = countries

	// 5. Save movie
	if err := db.Where("id = ?", data.Movie.ID).First(&movies.Movie{}).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		if err := db.Create(&data.Movie).Error; err != nil {
			log.Printf("Failed to save movie: %v", err)
		}
	}

	// 6. Save episodes (clean data, no nested movie)
	for _, group := range data.Episodes {
		for _, ep := range group.ServerData {
			ep.MovieID = data.Movie.ID
			ep.Movie = movies.Movie{} // Avoid circular serialization
			ep.ServerName = group.ServerName
			if err := db.Save(&ep).Error; err != nil {
				log.Printf("Failed to save episode %s: %v", ep.Name, err)
			}
		}
	}

	// 7. Assemble and cache
	result := map[string]interface{}{
		"status":   true,
		"msg":      "",
		"movie":    data.Movie,
		"episodes": data.Episodes,
	}
	jsonBytes, _ := json.Marshal(result)
	_ = rdb.Set(ctx, cacheKey, jsonBytes, 12*time.Hour).Err()

	return result, nil
}

func GetMovieDetailsFromDB(slug string) (map[string]interface{}, error) {
	db := config.DB
	rdb := config.RDB
	ctx := config.Ctx

	targetURL := fmt.Sprintf("https://phimapi.com/phim/%s", slug)
	cacheKey := fmt.Sprintf("details_movies:%s", targetURL)

	// 1. Find movie
	var movie movies.Movie
	if err := db.Where("slug = ?", slug).First(&movie).Error; err != nil {
		return nil, err
	}

	if err := db.Preload("Categories").Preload("Countries").Where("slug = ?", slug).First(&movie).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("movie not found in database: %s", slug)
		}
		return nil, err
	}

	// 2. Find episodes
	var allEpisodes []movies.Episode
	if err := db.Where("movie_id = ?", movie.ID).Find(&allEpisodes).Error; err != nil {
		return nil, fmt.Errorf("failed to load episodes: %v", err)
	}

	// 3. Group by server name
	grouped := make(map[string][]movies.Episode)
	for _, ep := range allEpisodes {
		ep.Movie = movies.Movie{} // Avoid embedding full movie in each episode
		grouped[ep.ServerName] = append(grouped[ep.ServerName], ep)
	}

	// 4. Format to []EpisodeGroup
	var episodeGroups []movies.EpisodeGroup
	for serverName, eps := range grouped {
		episodeGroups = append(episodeGroups, movies.EpisodeGroup{
			ServerName: serverName,
			ServerData: eps,
		})
	}

	// 5. Final result
	result := map[string]interface{}{
		"status":   true,
		"msg":      "",
		"movie":    movie,
		"episodes": episodeGroups,
	}

	// 6. Cache it
	if jsonStr, err := json.Marshal(result); err == nil {
		_ = rdb.Set(ctx, cacheKey, jsonStr, 12*time.Hour).Err()
	}

	return result, nil
}

// ListMoviesByCategory fetches movies by category from phimapi.com with Redis caching
func ListMoviesByCategory(req lib.MoviesByCategoryRequest) (map[string]interface{}, error) {
	rdb := config.RDB
	ctx := config.Ctx

	targetURL := buildCategoryURL(req)
	cacheKey := buildCategoryCacheKey(req)

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
		return nil, fmt.Errorf("failed to fetch category movies: %w", err)
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

	// 3. Cache the result
	var ttl time.Duration
	ttl = 16 * time.Hour
	jsonStr, _ := json.Marshal(apiResponse)
	_ = rdb.Set(ctx, cacheKey, jsonStr, ttl).Err()

	// 4. Track tag key for indexing
	tagKey := "movie_category:cached_keys"
	rdb.SAdd(ctx, tagKey, cacheKey)
	rdb.Expire(ctx, tagKey, 12*time.Hour)

	var returnMap map[string]interface{}
	_ = json.Unmarshal(jsonStr, &returnMap)

	return returnMap, nil
}

// buildCategoryURL constructs the phimapi.com URL for category browsing
func buildCategoryURL(req lib.MoviesByCategoryRequest) string {
	baseURL := fmt.Sprintf("https://phimapi.com/v1/api/the-loai/%s", req.Category)
	params := url.Values{}
	params.Add("page", fmt.Sprintf("%d", req.Page))
	params.Add("sort_field", req.SortField)
	params.Add("sort_type", req.SortType)
	params.Add("limit", fmt.Sprintf("%d", req.Limit))

	if req.SortLang != "" {
		params.Add("sort_lang", req.SortLang)
	}
	if req.Country != "" {
		params.Add("country", req.Country)
	}
	if req.Year != 0 {
		params.Add("year", fmt.Sprintf("%d", req.Year))
	}
	return baseURL + "?" + params.Encode()
}

// buildCategoryCacheKey builds a Redis cache key for category movie list
func buildCategoryCacheKey(req lib.MoviesByCategoryRequest) string {
	return fmt.Sprintf("movie_category:%s:%d:%s:%s:%s:%s:%d:%d",
		req.Category, req.Page, req.SortField, req.SortType,
		req.SortLang, req.Country, req.Year, req.Limit)
}

// ---------COUNTRY SIDE SERVICES---------------//
func ListMoviesByCountry(req lib.MoviesByCategoryRequest) (map[string]interface{}, error) {
	rdb := config.RDB
	ctx := config.Ctx

	targetURL := buildCountryURL(req)
	cacheKey := buildCountryCacheKey(req)

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
		return nil, fmt.Errorf("failed to fetch country movies: %w", err)
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

	// 3. Cache the result
	var ttl time.Duration
	ttl = 16 * time.Hour
	jsonStr, _ := json.Marshal(apiResponse)
	_ = rdb.Set(ctx, cacheKey, jsonStr, ttl).Err()

	// 4. Track tag key for indexing
	tagKey := "movie_country:cached_keys"
	rdb.SAdd(ctx, tagKey, cacheKey)
	rdb.Expire(ctx, tagKey, 12*time.Hour)

	var returnMap map[string]interface{}
	_ = json.Unmarshal(jsonStr, &returnMap)

	return returnMap, nil
}

func buildCountryURL(req lib.MoviesByCategoryRequest) string {
	baseURL := fmt.Sprintf("https://phimapi.com/v1/api/quoc-gia/%s", req.Country)
	params := url.Values{}
	params.Add("page", fmt.Sprintf("%d", req.Page))
	params.Add("sort_field", req.SortField)
	params.Add("sort_type", req.SortType)
	params.Add("limit", fmt.Sprintf("%d", req.Limit))

	if req.SortLang != "" {
		params.Add("sort_lang", req.SortLang)
	}
	if req.Country != "" {
		params.Add("country", req.Country)
	}
	if req.Year != 0 {
		params.Add("year", fmt.Sprintf("%d", req.Year))
	}
	return baseURL + "?" + params.Encode()
}

func buildCountryCacheKey(req lib.MoviesByCategoryRequest) string {
	return fmt.Sprintf("movie_country:%s:%d:%s:%s:%s:%s:%d:%d",
		req.Category, req.Page, req.SortField, req.SortType,
		req.SortLang, req.Country, req.Year, req.Limit)
}
