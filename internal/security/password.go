package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      = 19 * 1024
	argonIterations  = 2
	argonParallelism = 1
	argonSaltLength  = 16
	argonKeyLength   = 32
)

func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", errors.New("password must be at least 8 characters")
	}
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory,
		argonIterations,
		argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func VerifyPassword(encoded, password string) bool {
	params, salt, expected, err := parseHash(encoded)
	if err != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, params.time, params.memory, params.parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

type argonParams struct {
	memory      uint32
	time        uint32
	parallelism uint8
}

func parseHash(encoded string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return argonParams{}, nil, nil, errors.New("invalid password hash")
	}
	if parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return argonParams{}, nil, nil, errors.New("unsupported argon2 version")
	}

	paramParts := strings.Split(parts[3], ",")
	if len(paramParts) != 3 {
		return argonParams{}, nil, nil, errors.New("invalid argon2 params")
	}
	memory, err := parseUintParam(paramParts[0], "m")
	if err != nil {
		return argonParams{}, nil, nil, err
	}
	timeCost, err := parseUintParam(paramParts[1], "t")
	if err != nil {
		return argonParams{}, nil, nil, err
	}
	parallelism, err := parseUintParam(paramParts[2], "p")
	if err != nil {
		return argonParams{}, nil, nil, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argonParams{}, nil, nil, err
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return argonParams{}, nil, nil, err
	}
	return argonParams{
		memory:      uint32(memory),
		time:        uint32(timeCost),
		parallelism: uint8(parallelism),
	}, salt, hash, nil
}

func parseUintParam(part, key string) (uint64, error) {
	prefix := key + "="
	if !strings.HasPrefix(part, prefix) {
		return 0, errors.New("invalid argon2 parameter")
	}
	value, err := strconv.ParseUint(strings.TrimPrefix(part, prefix), 10, 32)
	if err != nil {
		return 0, err
	}
	return value, nil
}
