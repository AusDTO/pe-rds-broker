package utils_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/AusDTO/pe-rds-broker/utils"
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
})
