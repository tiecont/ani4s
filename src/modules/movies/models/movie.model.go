package movies

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

type Movie struct {
	ID             string         `json:"_id" gorm:"primaryKey;type:varchar(64)"`
	Name           string         `json:"name"`
	Slug           string         `json:"slug"`
	OriginName     string         `json:"origin_name"`
	Content        string         `json:"content" gorm:"type:text"`
	Type           string         `json:"type"`
	Status         string         `json:"status"`
	PosterURL      string         `json:"poster_url"`
	ThumbURL       string         `json:"thumb_url"`
	IsCopyright    bool           `json:"is_copyright"`
	SubDocQuyen    bool           `json:"sub_docquyen"`
	ChieuRap       bool           `json:"chieurap"`
	TrailerURL     string         `json:"trailer_url"`
	Time           string         `json:"time"`
	EpisodeCurrent string         `json:"episode_current"`
	EpisodeTotal   string         `json:"episode_total"`
	Quality        string         `json:"quality"`
	Lang           string         `json:"lang"`
	Notify         string         `json:"notify"`
	Showtimes      string         `json:"showtimes"`
	Year           int            `json:"year"`
	View           int            `json:"view"`
	Actor          pq.StringArray `json:"actor" gorm:"type:text[]"`
	Director       pq.StringArray `json:"director" gorm:"type:text[]"`

	TMDB     TMDBInfo  `json:"tmdb" gorm:"embedded"`
	IMDB     IMDBInfo  `json:"imdb" gorm:"embedded"`
	Created  Timestamp `json:"created" gorm:"embedded"`
	Modified Timestamp `json:"modified" gorm:"embedded"`

	Categories []Category `json:"category" gorm:"many2many:movie_categories;"`
	Countries  []Country  `json:"country" gorm:"many2many:movie_countries;"`
}

type TMDBInfo struct {
	Type        string  `json:"type"`
	ID          string  `json:"id"`
	Season      int     `json:"season"`
	VoteAverage float64 `json:"vote_average"`
	VoteCount   int     `json:"vote_count"`
}

type IMDBInfo struct {
	ID string `json:"id"`
}

type Timestamp struct {
	Time time.Time `json:"time"`
}

type Category struct {
	ID   string `json:"id" gorm:"primaryKey"`
	Name string `json:"name" gorm:"index"`
	Slug string `json:"slug" gorm:"index"`
}

type Country struct {
	ID   string `json:"id" gorm:"primaryKey"`
	Name string `json:"name" gorm:"index"`
	Slug string `json:"slug" gorm:"index"`
}

func MigrateMovies(db *gorm.DB) error {
	return db.AutoMigrate(&Movie{})
}
