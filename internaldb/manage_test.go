package internaldb_test

import (
	. "github.com/AusDTO/pe-rds-broker/internaldb"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/jinzhu/gorm"
	"code.cloudfoundry.org/lager"
	"github.com/AusDTO/pe-rds-broker/config"
	"code.cloudfoundry.org/lager/lagertest"
	"encoding/hex"
	"os"
	"fmt"
)

var _ = Describe("Models", func() {
	Context("RotateKey", func() {
		var (
			db *gorm.DB
			old_key, new_key []byte
			logger lager.Logger
			failFast bool
			instances []DBInstance
			users []DBUser
		)
		BeforeEach(func() {
			logger = lager.NewLogger("rdsbroker_test")
			logger.RegisterSink(lagertest.NewTestSink())
			var err error
			os.Remove("/tmp/test.sqlite3")
			db, err = DBInit(&config.DBConfig{DBType: "sqlite3", DBName: "/tmp/test.sqlite3"}, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(db).NotTo(BeNil())
			old_key, err = hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000000")
			Expect(err).NotTo(HaveOccurred())
			new_key, err = hex.DecodeString("0001020304050607080910111213141516171819202122232425262728293031")
			Expect(err).NotTo(HaveOccurred())
			failFast = true
			instances = []DBInstance{
				{
					InstanceID: "instance-1",
					Users: []DBUser{
						{Username: "user-1"},
						{Username: "user-2"},
					},
				},
				{
					InstanceID: "instance-2",
					Users: []DBUser{
						{Username: "user-3"},
						{Username: "user-4"},
					},
				},
			}
			for instance_i := range instances {
				for user_i := range instances[instance_i].Users {
					instances[instance_i].Users[user_i].SetPassword(fmt.Sprintf("password-%d-%d", instance_i, user_i), old_key)
				}
			}
			for _, instance := range instances {
				Expect(db.Create(&instance).Error).NotTo(HaveOccurred())
			}
			Expect(db.Find(&users).Error).NotTo(HaveOccurred())
		})

		Rotate := func() error {
			err := RotateKey(db, old_key, new_key, logger, failFast)
			Expect(db.Find(&users).Error).NotTo(HaveOccurred())
			return err
		}
		ExpectEncryptedOldKey := func() {
			for _, user := range users {
				Expect(user.Password(old_key)).To(MatchRegexp("password-\\d-\\d"))
				password, err := user.Password(new_key)
				Expect(err).To(MatchError("cipher: message authentication failed"))
				Expect(password).NotTo(MatchRegexp("password-\\d-\\d"))
			}
		}
		ExpectEncryptedNewKey := func() {
			for _, user := range users {
				password, err := user.Password(old_key)
				Expect(err).To(MatchError("cipher: message authentication failed"))
				Expect(password).NotTo(MatchRegexp("password-\\d-\\d"))
				Expect(user.Password(new_key)).To(MatchRegexp("password-\\d-\\d"))
			}
		}

		It("works in the normal case", func() {
			ExpectEncryptedOldKey()
			Expect(Rotate()).To(Succeed())
			ExpectEncryptedNewKey()
		})

		Context("if it's run twice", func() {
			var (
				err error
			)
			JustBeforeEach(func() {
				Expect(Rotate()).To(Succeed())
				err = Rotate()
			})
			It("gives a helpful error", func() {
				Expect(err).To(MatchError("cipher: message authentication failed"))
			})
			It("data is still valid", func() {
				ExpectEncryptedNewKey()
			})

			Context("without failFast", func() {
				BeforeEach(func() {
					failFast = false
				})
				It("reports the number of errors", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("completed with 4 errors"))
				})
			})
		})
	})
})
