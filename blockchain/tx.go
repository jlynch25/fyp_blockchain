package blockchain

import (
	"bytes"

	"github.com/jlynch25/golang-blockchain/wallet"
)

// TxOutput struct
type TxOutput struct {
	Value      int
	PubKeyHash []byte
}

// TxInput struct
type TxInput struct {
	ID        []byte
	Out       int
	Signature []byte
	PubKey    []byte
}

// NewTxOutput function
func NewTxOutput(value int, address string) *TxOutput {
	txo := &TxOutput{value, nil}
	txo.Lock([]byte(address))

	return txo
}

// UsesKey function
func (in *TxInput) UsesKey(pubKeyHash []byte) bool {
	lockingHash := wallet.PublicKeyHash(in.PubKey)

	return bytes.Compare(lockingHash, pubKeyHash) == 0
}

// Lock function
func (out *TxOutput) Lock(address []byte) {
	pubKeyHash := wallet.Base58Decode(address)
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]
	out.PubKeyHash = pubKeyHash
}

// IsLockedWithKey function
func (out *TxOutput) IsLockedWithKey(pubKeyHash []byte) bool {
	return bytes.Compare(out.PubKeyHash, pubKeyHash) == 0
}
