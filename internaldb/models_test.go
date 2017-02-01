package internaldb_test

import (
	. "github.com/AusDTO/pe-rds-broker/internaldb"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Models", func() {
	var (
		encryptionKey = make([]byte, 32)
		instanceID = "instance-id"
	)
	Describe("NewInstance", func() {
		It("creates a master user", func() {
			instance, err := NewInstance(instanceID, encryptionKey)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(instance.Users)).To(Equal(1))
			Expect(instance.Users[0].Type).To(Equal(Master))
		})

		It("errors with bad encryption key", func () {
			_, err := NewInstance(instanceID, make([]byte, 3))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("NewUser", func () {
		Context("has random", func () {
			var (
				one, two DBUser
			)
			BeforeEach(func () {
				var err error
				one, err = NewUser(Standard, encryptionKey)
				Expect(err).NotTo(HaveOccurred())
				two, err = NewUser(Standard, encryptionKey)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Username", func() {
				Expect(one.Username).NotTo(Equal(two.Username))
			})

			It("EncryptedPassword", func() {
				Expect(one.EncryptedPassword).NotTo(Equal(two.EncryptedPassword))
			})

			It("IV", func() {
				Expect(one.IV).NotTo(Equal(two.IV))
			})

			It("Password", func() {
				pass1, err := one.Password(encryptionKey)
				Expect(err).NotTo(HaveOccurred())
				pass2, err := two.Password(encryptionKey)
				Expect(err).NotTo(HaveOccurred())
				Expect(pass1).NotTo(Equal(pass2))
			})
		})

		It("errors with bad encryption key", func () {
			_, err := NewUser(Standard, make([]byte, 3))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("find user methods", func () {
		var (
			masterUser = DBUser{Type: Master, Username: "master"}
			bindingUser1 = DBUser{Type: Standard, BindingID: "one", Username: "bind1"}
			bindingUser2 = DBUser{Type: Standard, BindingID: "two", Username: "bind2"}
			instance = DBInstance{Users: []DBUser{bindingUser1, masterUser, bindingUser2}}
		)

		Describe("MasterUser", func () {
			It("finds the right user", func() {
				Expect(instance.MasterUser()).To(Equal(&masterUser))
			})
		})

		Describe("BindingUser", func() {
			It("finds binding 1", func() {
				Expect(instance.BindingUser("one")).To(Equal(&bindingUser1))
			})

			It("finds binding 2", func() {
				Expect(instance.BindingUser("two")).To(Equal(&bindingUser2))
			})

			It("doesn't find binding 3", func() {
				Expect(instance.BindingUser("three")).To(BeNil())
			})
		})
	})
})
