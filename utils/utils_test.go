package utils_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/AusDTO/pe-rds-broker/utils"
)

var _ = Describe("RandomAlphaNum", func() {
	It("generates a random alpha numeric with the proper length", func() {
		randomString := RandomAlphaNum(32)
		Expect(len(randomString)).To(Equal(32))
	})
})

var _ = Describe("GetMD5B64", func() {
	It("returns the Base64 of a string MD5", func() {
		md5b64 := GetMD5B64("ce71b484-d542-40f7-9dd4-5526e38c81ba", 32)
		Expect(md5b64).To(Equal("Y2U3MWI0ODQtZDU0Mi00MGY3LTlkZDQt"))
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
