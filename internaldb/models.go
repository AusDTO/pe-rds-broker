package internaldb

import (
	"time"
	"github.com/AusDTO/pe-rds-broker/utils"
	"database/sql/driver"
	"github.com/jinzhu/gorm"
	"errors"
	"fmt"
	"strings"
)

type DBInstance struct {
	// Managed by gorm
	ID uint64 `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time
	// Managed by us
	InstanceID string `gorm:"unique_index"`
	DBName string
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
	Bindings []DBBinding
}

type DBBinding struct {
	ID uint64 `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time
	BindingID string
	DBUser DBUser // gorm belongs to relationship
	DBUserID uint64
}

type DBUserType string
const (
	Master DBUserType = "master"
	SuperUser DBUserType = "superuser"
	Standard DBUserType = "standard"
)

// Remember to DB.Save() from the caller
func NewInstance(instanceID, dbPrefix string, key []byte) (*DBInstance, error) {
	instance := DBInstance{
		InstanceID: instanceID,
		Users: make([]DBUser, 1),
		DBName: fmt.Sprintf("%s_%s", dbPrefix, strings.Replace(instanceID, "-", "_", -1)),
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
	err := db.Where(&DBInstance{InstanceID: instanceID}).Preload("Users.Bindings").First(&instance).Error
	if err != nil {
		return nil
	}
	return &instance
}

func (i *DBInstance) Bind(db *gorm.DB, bindingID, username string, userType DBUserType, key []byte) (user DBUser, new bool, err error) {
	current_user := i.User(username)
	if current_user == nil {
		new = true
		user, err = i.NewUser(userType, key)
		if err != nil {
			return
		}
		user.Username = username
	} else {
		new = false
		user = *current_user
	}
	user.Bindings = append(user.Bindings, DBBinding{BindingID: bindingID})
	err = db.Save(&user).Error
	if err != nil {
		return
	}
	return
}

func (i *DBInstance) Unbind(db *gorm.DB, bindingID string) (user DBUser, delete bool, err error) {
	user_p, binding := i.BindingUser(bindingID)
	if user_p == nil || binding == nil {
		return user, false, errors.New("Unknown binding ID")
	}
	user = *user_p
	// delete if this is the last binding
	delete = len(user.Bindings) == 1
	err = db.Delete(&binding).Error
	if err != nil {
		return
	}
	return
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
		err = user.Delete(db)
		if err != nil {
			return err
		}
	}
	return db.Delete(i).Error
}

func (u *DBUser) Delete(db *gorm.DB) error {
	var err error
	for _, binding := range u.Bindings {
		err = db.Delete(&binding).Error
		if err != nil {
			return err
		}
	}
	return db.Delete(u).Error
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

func (i *DBInstance) BindingUser(bindingID string) (*DBUser, *DBBinding) {
	for _, user := range i.Users {
		if user.Type == Standard {
			for _, binding := range user.Bindings {
				if binding.BindingID == bindingID {
					return &user, &binding
				}
			}
		}
	}
	return nil, nil
}

func (i *DBInstance) User(username string) *DBUser {
	for _, user := range i.Users {
		if user.Username == username {
			return &user
		}
	}
	return nil
}

func (i *DBInstance) UnderscoreID() string {
	return strings.Replace(i.InstanceID, "-", "_", -1)
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

