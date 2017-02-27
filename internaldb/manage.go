package internaldb

import (
	"github.com/jinzhu/gorm"
	"code.cloudfoundry.org/lager"
	"fmt"
)

func RotateKey(db *gorm.DB, old_key, new_key []byte, logger lager.Logger, failFast bool) error {
	var instances []DBInstance
	err_count := 0
	err := db.Preload("Users").Find(&instances).Error
	if err != nil {
		logger.Fatal("get-instances", err)
	}
	RotateOne := func(instance DBInstance, user DBUser) error {
		password, err := user.Password(old_key)
		if err != nil {
			logger.Error("decrypt-password", err, lager.Data{"instance": instance.InstanceID, "user": user.Username})
			return err
		}
		err = user.SetPassword(password, new_key)
		if err != nil {
			logger.Error("encrypt-password", err, lager.Data{"instance":instance.InstanceID, "user":user.Username})
			return err
		}
		err = db.Save(&user).Error
		if err != nil {
			logger.Error("save-password", err, lager.Data{"instance":instance.InstanceID, "user":user.Username})
			return err
		}
		return err
	}
	for _, instance := range instances {
		for _, user := range instance.Users {
			err = RotateOne(instance, user)
			if err != nil {
				if failFast {
					return err
				} else {
					err_count += 1
				}
			}
		}
	}
	if err_count != 0 {
		return fmt.Errorf("Key rotation completed with %d errors. See the logs for more details.", err_count)
	}
	return nil
}
