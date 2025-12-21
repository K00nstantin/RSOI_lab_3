package models

import (
	"time"
)

type Rating struct {
	ID        uint   `gorm:"primaryKey"`
	Username  string `gorm:"size:80;not null;uniqueIndex"`
	Stars     int    `gorm:"not null;check:stars >= 0 AND stars <= 100"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Library struct {
	ID         uint   `gorm:"primaryKey"`
	LibraryUid string `gorm:"type:uuid;uniqueIndex;not null"`
	Name       string `gorm:"size:80;not null"`
	City       string `gorm:"not null"`
	Address    string `gorm:"not null"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Book struct {
	ID        uint   `gorm:"primaryKey"`
	BookUid   string `gorm:"type:uuid;uniqueIndex;not null"`
	Name      string `gorm:"not null"`
	Author    string
	Genre     string
	Condition string `gorm:"size:20;default:'EXCELLENT'"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type LibraryBook struct {
	ID             uint `gorm:"primaryKey"`
	LibraryID      uint
	BookID         uint
	AvailableCount int `gorm:"not null"`

	Library Library `gorm:"foreignKey:LibraryID"`
	Book    Book    `gorm:"foreignKey:BookID"`
}

type Reservation struct {
	ID             uint   `gorm:"primaryKey"`
	ReservationUid string `gorm:"type:uuid;uniqueIndex;not null"`
	Username       string `gorm:"size:80;not null"`
	BookUid        string `gorm:"type:uuid;not null"`
	LibraryUid     string `gorm:"type:uuid;not null"`
	Status         string `gorm:"size:20;not null"`
	BookCondition  string `gorm:"size:20"` // Состояние книги на момент выдачи
	StartDate      time.Time
	TillDate       time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
