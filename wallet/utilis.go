package wallet

import (
	"github.com/mr-tron/base58"
)

// Base58Encode function
func Base58Encode(input []byte) []byte {
	encode := base58.Encode(input)

	return []byte(encode)
}

// Base58Decode function
func Base58Decode(input []byte) []byte {
	decode, err := base58.Decode(string(input[:]))
	Handle(err)

	return decode
}

// the missing chanacters are 		O 0 l I + /
