package wallet

import (
	"bytes"
	"crypto/elliptic"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
)

const (
	// basePath = "/data/data/com.github.jlynch25.mylib_example/files"
	// basePath = "/Internal storage/storage/emulated/0"
	basePath   = "/data/user/0/com.github.jlynch25.mylib_example/app_flutter"
	walletFile = basePath + "/tmp/wallets_%s.data"
	// walletFile = "/tmp/wallets_%s.data"
)

// Wallets struct
type Wallets struct {
	Wallets map[string]*Wallet
}

// CreateWallets function
func CreateWallets(nodeID string) (*Wallets, error) {
	wallets := Wallets{}
	wallets.Wallets = make(map[string]*Wallet)

	err := wallets.LoadFile(nodeID)

	return &wallets, err
}

// AddWallet function
func (ws *Wallets) AddWallet() string {
	wallet := MakeWallet()
	address := fmt.Sprintf("%s", wallet.Address())

	ws.Wallets[address] = wallet

	return address
}

// GetAllAddresses function
func (ws *Wallets) GetAllAddresses() []string {
	var addresses []string

	for address := range ws.Wallets {
		addresses = append(addresses, address)
	}

	return addresses
}

// GetWallet function
func (ws *Wallets) GetWallet(address string) Wallet {
	return *ws.Wallets[address]
}

// LoadFile function
func (ws *Wallets) LoadFile(nodeID string) error {
	if _, err := os.Stat(basePath + "tmp"); os.IsNotExist(err) {
		os.Mkdir(basePath+"tmp", 0755)
	}
	walletFile := fmt.Sprintf(walletFile, nodeID)
	if _, err := os.Stat(walletFile); os.IsNotExist(err) {
		return err
	}

	var wallets Wallets

	fileContent, err := ioutil.ReadFile(walletFile)

	gob.Register(elliptic.P256())
	decoder := gob.NewDecoder(bytes.NewReader(fileContent))
	err = decoder.Decode(&wallets)
	Handle(err)

	ws.Wallets = wallets.Wallets

	return nil
}

// SaveFile function
func (ws *Wallets) SaveFile(nodeID string) {
	var content bytes.Buffer
	walletFile := fmt.Sprintf(walletFile, nodeID)

	gob.Register(elliptic.P256())

	encoder := gob.NewEncoder(&content)
	err := encoder.Encode(ws)
	Handle(err)

	os.Create(walletFile)
	err = ioutil.WriteFile(walletFile, content.Bytes(), 0644)
	Handle(err)
}
