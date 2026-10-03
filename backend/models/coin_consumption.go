package models

// Preserves original expiry when a redemption is refunded.
type CoinConsumption struct {
	ID          uint   `gorm:"primaryKey"`
	UserID      uint   `gorm:"not null;index"`
	BatchID     uint   `gorm:"not null"`
	Kind        string `gorm:"not null;index:idx_consumption_ref"`
	ReferenceID uint   `gorm:"not null;index:idx_consumption_ref"`
	Amount      int    `gorm:"not null"`
	Refunded    bool
}
