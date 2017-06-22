package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"log"
	"os"
	"regexp"
	"strings"

	"code.cloudfoundry.org/lager"
)

// Must be a multiple of 4
const PasswordLength = 24

// Must be a multiple of 4 plus 1
// Must be <= 16 because mysql
const UsernameLength = 13

func Encrypt(msg string, key, iv []byte) ([]byte, error) {
	src := []byte(msg)

	aesBlock, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(aesBlock)
	if err != nil {
		return nil, err
	}
	return aesgcm.Seal(nil, iv, src, nil), nil
}

func Decrypt(src, key, iv []byte) (string, error) {
	aesBlock, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aesgcm, err := cipher.NewGCM(aesBlock)
	if err != nil {
		return "", err
	}
	dst, err := aesgcm.Open(nil, iv, src, nil)
	if err != nil {
		return "", err
	}
	return string(dst), nil
}

func RandIV() ([]byte, error) {
	// 12 bytes is the standard nonce size for GCM
	var bytes = make([]byte, 12)
	_, err := rand.Read(bytes)
	return bytes, err
}

func RandString(length int) (string, error) {
	var bytes = make([]byte, length*3/4)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

func RandPassword() (string, error) {
	return RandString(PasswordLength)
}

func RandUsername() (string, error) {
	// Username must start with a letter and can't have hyphens
	username, err := RandString(UsernameLength - 1)
	if err != nil {
		return "", err
	}
	return "u" + strings.Replace(username, "-", "_", -1), nil
}

func IsSimpleIdentifier(arg string) bool {
	return regexp.MustCompile("^$|^[[:alpha:]][_[:alnum:]]*$").MatchString(arg)
}

func IsValidExtensionName(arg string) bool {
	return regexp.MustCompile("^[[:alpha:]][-_[:alnum:]]*$").MatchString(arg)
}

func BuildLogger(logLevel, component string) lager.Logger {
	logLevels := map[string]lager.LogLevel{
		"DEBUG": lager.DEBUG,
		"INFO":  lager.INFO,
		"ERROR": lager.ERROR,
		"FATAL": lager.FATAL,
	}
	lagerLogLevel, ok := logLevels[strings.ToUpper(logLevel)]
	if !ok {
		log.Fatal("Invalid log level: ", logLevel)
	}

	logger := lager.NewLogger(component)
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lagerLogLevel))

	return logger
}
