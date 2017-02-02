package utils_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/AusDTO/pe-rds-broker/utils"
	"strings"
)

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
			Expect(username).To(MatchRegexp("^[[:alpha:]][_[:alnum:]]*$"))
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
