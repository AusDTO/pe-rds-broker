package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"io"
	"math"
)

// Must be a multiple of 4
const PasswordLength = 24
const UsernameLength = 24

var alpha = []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz")
var numer = []byte("0123456789")

func RandomAlphaNum(length int) string {
	return randChar(1, alpha) + randChar(length-1, append(alpha, numer...))
}

func randChar(length int, chars []byte) string {
	newPword := make([]byte, length)
	randomData := make([]byte, length+(length/4))
	clen := byte(len(chars))
	maxrb := byte(256 - (256 % len(chars)))
	i := 0
	for {
		if _, err := io.ReadFull(rand.Reader, randomData); err != nil {
			panic(err)
		}
		for _, c := range randomData {
			if c >= maxrb {
				continue
			}
			newPword[i] = chars[c%clen]
			i++
			if i == length {
				return string(newPword)
			}
		}
	}
}

func GetMD5B64(text string, length int) string {
	hasher := md5.New()
	md5 := hasher.Sum([]byte(text))
	return base64.StdEncoding.EncodeToString(md5)[0:int(math.Min(float64(length), float64(len(md5))))]
}

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
