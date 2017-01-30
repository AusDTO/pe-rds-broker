package utils_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/AusDTO/pe-rds-broker/utils"
	"gopkg.in/yaml.v2"
	"os"
	"io/ioutil"
	"encoding/hex"
)

type HexTestVector struct {
	Count int
	Key string
	IV string
	Plaintext string
	Ciphertext string
}

type TestVectors struct {
	Encrypt []HexTestVector
	Decrypt []HexTestVector
}

type TestVector struct {
	Count int
	Key []byte
	IV []byte
	Plaintext []byte
	Ciphertext []byte
}

func GetTestVectors() TestVectors {
	file, err := os.Open("test_vectors.yml")
	Expect(err).NotTo(HaveOccurred())
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	Expect(err).NotTo(HaveOccurred())

	var testVectors TestVectors
	Expect(err).NotTo(HaveOccurred())
	err = yaml.Unmarshal(bytes, &testVectors)
	Expect(err).NotTo(HaveOccurred())
	return testVectors
}

func (src *HexTestVector) Decode() TestVector {
	var dst TestVector
	dst.Count = src.Count
	dst.Key, _ = hex.DecodeString(src.Key)
	dst.IV, _ = hex.DecodeString(src.IV)
	dst.Plaintext, _ = hex.DecodeString(src.Plaintext)
	dst.Ciphertext, _ = hex.DecodeString(src.Ciphertext)
	return dst
}

var _ = Describe("Encrypt", func() {
	It("Matches the NIST test vectors", func() {
		testVectors := GetTestVectors()
		for _, hexTestVector := range testVectors.Encrypt {
			testVector := hexTestVector.Decode()
			ciphertext, err := Encrypt(string(testVector.Plaintext), testVector.Key, testVector.IV)
			Expect(err).NotTo(HaveOccurred())
			Expect(ciphertext).To(Equal(testVector.Ciphertext))
		}
	})
})

var _ = Describe("Decrypt", func() {
	It("Matches the NIST test vectors", func() {
		testVectors := GetTestVectors()
		for _, hexTestVector := range testVectors.Decrypt {
			testVector := hexTestVector.Decode()
			plaintext, err := Decrypt(testVector.Ciphertext, testVector.Key, testVector.IV)
			Expect(err).NotTo(HaveOccurred())
			Expect(plaintext).To(Equal(string(testVector.Plaintext)))
		}
	})
})
