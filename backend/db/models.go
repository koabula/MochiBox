package db

import (
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type File struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	CID            string         `gorm:"column:cid;index" json:"cid"`
	Name           string         `json:"name"`
	Size           int64          `json:"size"`
	MimeType       string         `json:"mime_type"`
	CreatedAt      time.Time      `json:"created_at"`
}

type SharedFile struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CID       string    `gorm:"column:cid" json:"cid"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	MimeType  string    `json:"mime_type"`
	CreatedAt time.Time `json:"created_at"`
}

type Settings struct {
	ID              uint   `gorm:"primaryKey" json:"id"`
	DownloadPath    string `json:"download_path"`
	AskPath         bool   `json:"ask_path"`
	IpfsApiUrl      string `json:"ipfs_api_url"`
	IpfsGatewayUrl  string `json:"ipfs_gateway_url"`
	UseEmbeddedNode bool   `json:"use_embedded_node"`
}

func InitDB(path string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&File{}, &Settings{}, &SharedFile{})
	if err != nil {
		return nil, err
	}

	// Initialize default settings if not exists
	var count int64
	db.Model(&Settings{}).Count(&count)
	if count == 0 {
		db.Create(&Settings{
			DownloadPath:    "",
			UseEmbeddedNode: true, // Default to true for new users
		})
	}

	return db, nil
}
