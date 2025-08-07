package movies

import "gorm.io/gorm"

type MovieDetails struct {
	Status   bool           `json:"status" gorm:"-"`
	Msg      string         `json:"msg" gorm:"-"`
	Movie    Movie          `json:"movie"`
	Episodes []EpisodeGroup `json:"episodes" gorm:"-"`
}

type EpisodeGroup struct {
	ServerName string    `json:"server_name"`
	ServerData []Episode `json:"server_data"`
}

type Episode struct {
	ID         uint   `json:"id" gorm:"primaryKey"`
	MovieID    string `json:"-" gorm:"not null;"`
	Movie      Movie  `json:"-" gorm:"foreignKey:MovieID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	ServerName string `json:"-" json:"server_name"`
	Name       string `json:"name"`
	Slug       string `json:"slug" gorm:"uniqueIndex;type:varchar(255)"`
	Filename   string `json:"filename"`
	LinkEmbed  string `json:"link_embed"`
	LinkM3U8   string `json:"link_m3u8"`
}

func MigrateMovieDetails(db *gorm.DB) error {
	if err := db.AutoMigrate(&Episode{}); err != nil {
		return err
	}
	return nil
}
