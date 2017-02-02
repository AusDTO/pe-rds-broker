package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
)

// Must be a multiple of 4
const PasswordLength = 24
const UsernameLength = 24

func Encrypt(msg string, key, iv []byte) ([]byte, error) {
	src := []byte(msg)
	var dst []byte

	aesBlockEncrypter, err := aes.NewCipher(key)
	if err != nil {
		return dst, err
	}

	dst = make([]byte, len(src))
	aesEncrypter := cipher.NewCFBEncrypter(aesBlockEncrypter, iv)
	aesEncrypter.XORKeyStream(dst, src)

	return dst, nil
}

func Decrypt(src, key, iv []byte) (string, error) {
	dst := make([]byte, len(src))

	aesBlockDecrypter, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesDecrypter := cipher.NewCFBDecrypter(aesBlockDecrypter, iv)
	aesDecrypter.XORKeyStream(dst, src)

	return string(dst), nil
}

func RandIV() ([]byte, error) {
	var bytes = make([]byte, aes.BlockSize)
	_, err := rand.Read(bytes)
	return bytes, err
}

func RandString(length int) (string, error) {
	var bytes = make([]byte, length*3/4)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

func RandPassword() (string, error) {
	return RandString(PasswordLength)
}

func RandUsername() (string, error) {
	return RandString(UsernameLength)
}
