package migration

import (
	"github.com/Hhz0823/1s-ui/util"

	"gorm.io/gorm"
)

// to1_5_2 hashes plaintext admin passwords stored in older databases.
func to1_5_2(tx *gorm.DB) error {
	type userRow struct {
		Id       uint
		Password string
	}
	var users []userRow
	if err := tx.Raw("SELECT id, password FROM users").Scan(&users).Error; err != nil {
		return err
	}
	for _, u := range users {
		if u.Password == "" || util.IsHashedPassword(u.Password) {
			continue
		}
		hashed, err := util.HashPassword(u.Password)
		if err != nil {
			return err
		}
		if err := tx.Exec("UPDATE users SET password = ? WHERE id = ?", hashed, u.Id).Error; err != nil {
			return err
		}
	}
	return nil
}
