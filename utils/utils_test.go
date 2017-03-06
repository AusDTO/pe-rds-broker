package utils_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/AusDTO/pe-rds-broker/utils"
	"strings"
)

var _ = Describe("UsernameLength", func() {
	It("is valid", func() {
		// yay mysql
		Expect(UsernameLength).To(BeNumerically("<=", 16))
	})
})

var _ = Describe("RandIV", func() {
	It("returns a random result", func() {
		one, err := RandIV()
		Expect(err).NotTo(HaveOccurred())
		two, err := RandIV()
		Expect(err).NotTo(HaveOccurred())
		Expect(one).NotTo(Equal(two))
	})
})

var _ = Describe("RandUsername", func() {
	It("returns a random result", func() {
		one, err := RandUsername()
		Expect(err).NotTo(HaveOccurred())
		two, err := RandUsername()
		Expect(err).NotTo(HaveOccurred())
		Expect(one).NotTo(Equal(two))
	})

	It("returns the correct length", func() {
		username, err := RandUsername()
		Expect(err).NotTo(HaveOccurred())
		Expect(len(username)).To(Equal(UsernameLength))
	})

	It("is a valid username", func() {
		for i := 0; i < 10; i++ {
			username, err := RandUsername()
			Expect(err).NotTo(HaveOccurred())
			Expect(IsSimpleIdentifier(username)).To(BeTrue())
		}
	})
})

var _ = Describe("RandPassword", func() {
	It("returns a random result", func() {
		one, err := RandPassword()
		Expect(err).NotTo(HaveOccurred())
		two, err := RandPassword()
		Expect(err).NotTo(HaveOccurred())
		Expect(one).NotTo(Equal(two))
	})

	It("returns the correct length", func() {
		password, err := RandPassword()
		Expect(err).NotTo(HaveOccurred())
		Expect(len(password)).To(Equal(PasswordLength))
	})

	It("is a valid password", func() {
		for i := 0; i < 10; i++ {
			password, err := RandPassword()
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.ContainsAny(password, "/@\" ")).To(BeFalse(),
				"Password shouldn't contain special characters %s", password)
		}
	})
})

var _ = Describe("IsSimpleIdentifier", func() {
	It("allows valid strings", func() {
		Expect(IsSimpleIdentifier("hi")).To(BeTrue())
		Expect(IsSimpleIdentifier("hi123")).To(BeTrue())
		Expect(IsSimpleIdentifier("HI")).To(BeTrue())
		Expect(IsSimpleIdentifier("hi_there")).To(BeTrue())
		Expect(IsSimpleIdentifier("")).To(BeTrue())
	})

	It("rejects invalid strings", func() {
		Expect(IsSimpleIdentifier("*")).To(BeFalse())
		Expect(IsSimpleIdentifier("123hi")).To(BeFalse())
	})
})

var _ = Describe("IsValidExtensionName", func() {
	It("allows valid strings", func() {
		Expect(IsValidExtensionName("hi")).To(BeTrue())
		Expect(IsValidExtensionName("hi123")).To(BeTrue())
		Expect(IsValidExtensionName("HI")).To(BeTrue())
		Expect(IsValidExtensionName("hi_there")).To(BeTrue())
		Expect(IsValidExtensionName("uuid-ossp")).To(BeTrue())
	})

	It("rejects invalid strings", func() {
		Expect(IsValidExtensionName("")).To(BeFalse())
		Expect(IsValidExtensionName("*")).To(BeFalse())
		Expect(IsValidExtensionName("123hi")).To(BeFalse())
	})
})

var _ = Describe("DBUsername", func() {
	var (
		requestedUsername string
		instanceID string
		appID string
		engine string
		shared bool
		username string
	)
	BeforeEach(func() {
		requestedUsername = ""
		instanceID = "instance-id"
		appID = ""
		engine = ""
		shared = false
	})
	JustBeforeEach(func() {
		username = DBUsername(requestedUsername, instanceID, appID, engine, shared)
	})
	Context("as postgres", func() {
		BeforeEach(func() {
			engine = "postgres"
		})

		Context("dedicated instance", func() {
			Context("with requestedUsername", func(){
				BeforeEach(func() {
					requestedUsername = "custom"
				})

				It("gives the right username", func() {
					Expect(username).To(Equal("custom"))
				})
			})

			Context("without requestedUsername but with appID", func(){
				BeforeEach(func() {
					appID = "app-id"
				})

				It("gives the right username", func() {
					Expect(username).To(Equal("uapp_id"))
				})
			})

			Context("without requestedUsername or appID", func() {
				It("gives a random username", func() {
					Expect(username).To(HaveLen(UsernameLength))
				})
			})
		})

		Context("shared instance", func() {
			BeforeEach(func() {
				shared = true
			})

			Context("with requestedUsername", func(){
				BeforeEach(func() {
					requestedUsername = "custom"
				})

				It("gives the right username", func() {
					Expect(username).To(Equal("custom_instance_id"))
				})
			})

			Context("without requestedUsername but with appID", func(){
				BeforeEach(func() {
					appID = "app-id"
				})

				It("gives the right username", func() {
					Expect(username).To(Equal("uapp_id_instance_id"))
				})
			})

			Context("without requestedUsername or appID", func() {
				It("gives a random username with instanceID", func() {
					Expect(username).To(HaveSuffix("_instance_id"))
					Expect(username).To(HaveLen(UsernameLength + len("_instance_id")))
				})
			})

			Context("with a long username", func() {
				BeforeEach(func() {
					appID = "00000000-0000-0000-0000-000000000000"
					instanceID = "00000000-0000-0000-0000-000000000000"
				})
				It("truncates", func() {
					Expect(username).To(HaveLen(63))
				})
			})
		})
	})

	Context("as mysql", func() {
		BeforeEach(func() {
			engine = "mysql"
		})

		Context("with requestedUsername", func(){
			BeforeEach(func() {
				requestedUsername = "custom"
			})

			It("gives a random username", func() {
				Expect(username).To(HaveLen(UsernameLength))
			})
		})
	})
})
