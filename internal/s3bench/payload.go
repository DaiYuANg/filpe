package s3bench

import (
	"crypto/sha256"
	"strconv"
)

func deterministicBytes(size, seed int) []byte {
	data := make([]byte, size)
	digest := sha256.Sum256([]byte(strconv.Itoa(seed)))
	for index := range data {
		data[index] = digest[index%len(digest)]
	}
	return data
}
