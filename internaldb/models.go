package internaldb

import (
	"time"
	"github.com/AusDTO/pe-rds-broker/utils"
	"database/sql/driver"
	"github.com/jinzhu/gorm"
)

type DBInstance struct {
	// Managed by gorm
	ID uint64 `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time
	// Managed by us
	InstanceID string `gorm:"unique_index"`
	Users []DBUser

}

type DBUser struct {
	ID uint64 `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DBInstance DBInstance // gorm belongs to relationship
	DBInstanceID uint64
	Username string
	EncryptedPassword []byte
	IV []byte
	Type DBUserType
	BindingID string
}

type DBUserType string
const (
	Master DBUserType = "master"
	SuperUser DBUserType = "superuser"
	Standard DBUserType = "standard"
)

// Remember to DB.Save() from the caller
func NewInstance(instanceID string, key []byte) (*DBInstance, error) {
	instance := DBInstance{
		InstanceID: instanceID,
		Users: make([]DBUser, 1),
	}
	var err error
	instance.Users[0], err = NewUser(Master, key)
	if err != nil {
		return &instance, err
	}
	return &instance, nil
}

// Use this wrapper so we always preload the users
func FindInstance(db *gorm.DB, instanceID string) *DBInstance {
	var instance DBInstance
	err := db.Where(&DBInstance{InstanceID: instanceID}).Preload("Users").First(&instance).Error
	if err != nil {
		return nil
	}
	return &instance
}

func (i *DBInstance) NewUser(userType DBUserType, key []byte) (DBUser, error) {
	user, err := NewUser(userType, key)
	if err != nil {
		return user, err
	}
	user.DBInstance = *i
	i.Users = append(i.Users, user)
	return user, nil
}

func NewUser(userType DBUserType, key []byte) (DBUser, error) {
	var err error
	user := DBUser{Type: userType}
	user.Username, err = utils.RandUsername()
	if err != nil {
		return user, err
	}
	err = user.SetRandomPassword(key)
	if err != nil {
		return user, err
	}
	return user, nil
}

func (i *DBInstance) Delete(db *gorm.DB) error {
	var err error
	for _, user := range i.Users {
		err = db.Delete(&user).Error
		if err != nil {
			return err
		}
	}
	return db.Delete(i).Error
}

func (u *DBUser) SetPassword(password string, key []byte) error {
	iv, err := utils.RandIV()
	if err != nil {
		return err
	}
	encrypted, err := utils.Encrypt(password, key, iv)
	if err != nil {
		return err
	}
	u.EncryptedPassword = encrypted
	u.IV = iv
	return nil
}

func (u *DBUser) SetRandomPassword(key []byte) error {
	password, err := utils.RandPassword()
	if err != nil {
		return err
	}
	return u.SetPassword(password, key)
}

func (u *DBUser) Password(key []byte) (string, error) {
	return utils.Decrypt(u.EncryptedPassword, key, u.IV)
}

// We currently preload all users from the database and do the search
// in go code. We could also load the ones we need on demand using appropriate
// database queries. Given the number of users is expected to be small
// (usually just one Master and one Standard) the preload way seems fine for now.
func (i *DBInstance) MasterUser() *DBUser {
	for _, user := range i.Users {
		if user.Type == Master {
			return &user
		}
	}
	return nil
}

func (i *DBInstance) BindingUser(bindingID string) *DBUser {
	for _, user := range i.Users {
		if user.Type == Standard && user.BindingID == bindingID {
			return &user
		}
	}
	return nil
}

// Ensure custom string type works with gorm/sql
// https://github.com/jinzhu/gorm/issues/302
func (u *DBUserType) Scan(value interface{}) error {
	*u = DBUserType(value.([]byte))
	return nil
}

func (u DBUserType) Value() (driver.Value, error)  {
	return string(u), nil
}

