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
		dbPrefix = "cf"
		serviceID = "service-id"
		planID = "plan-id"
	)
	Describe("NewInstance", func() {
		It("creates a master user", func() {
			instance, err := NewInstance(serviceID, planID, instanceID, dbPrefix, encryptionKey)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(instance.Users)).To(Equal(1))
			Expect(instance.Users[0].Type).To(Equal(Master))
			Expect(instance.DBName).To(Equal("cf_instance_id"))
			Expect(instance.ServiceID).To(Equal("service-id"))
			Expect(instance.PlanID).To(Equal("plan-id"))
			Expect(instance.InstanceID).To(Equal("instance-id"))
		})

		It("errors with bad encryption key", func () {
			_, err := NewInstance(serviceID, planID, instanceID, dbPrefix, make([]byte, 3))
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
			binding1 = DBBinding{BindingID: "one"}
			binding2 = DBBinding{BindingID: "two"}
			binding3 = DBBinding{BindingID: "three"}
			bindingUser1 = DBUser{Type: Standard, Username: "bind1", Bindings: []DBBinding{binding1}}
			bindingUser2 = DBUser{Type: Standard, Username: "bind2", Bindings: []DBBinding{binding2, binding3}}
			instance = DBInstance{Users: []DBUser{bindingUser1, masterUser, bindingUser2}}
		)

		Describe("MasterUser", func () {
			It("finds the right user", func() {
				Expect(instance.MasterUser()).To(Equal(&masterUser))
			})
		})

		Describe("BindingUser", func() {
			It("finds binding 1", func() {
				user, binding := instance.BindingUser("one")
				Expect(user).To(Equal(&bindingUser1))
				Expect(binding).To(Equal(&binding1))
			})

			It("finds binding 3", func() {
				user, binding := instance.BindingUser("three")
				Expect(user).To(Equal(&bindingUser2))
				Expect(binding).To(Equal(&binding3))
			})

			It("doesn't find binding 4", func() {
				user, binding := instance.BindingUser("four")
				Expect(user).To(BeNil())
				Expect(binding).To(BeNil())
			})
		})

		Describe("User", func() {
			It("finds bind1", func() {
				Expect(instance.User("bind1")).To(Equal(&bindingUser1))
			})

			It("finds bind2", func() {
				Expect(instance.User("bind2")).To(Equal(&bindingUser2))
			})

			It("doesn't find bind3", func() {
				Expect(instance.User("bind3")).To(BeNil())
			})
		})
	})
})
