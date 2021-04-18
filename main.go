package theBlockchain

import (
	"fmt"
	"log"

	// "runtime/debug"
	"strconv"

	"github.com/jlynch25/golang-blockchain/blockchain"
	network "github.com/jlynch25/golang-blockchain/noise_network"
	"github.com/jlynch25/golang-blockchain/wallet"
)

// FIXME
func StartNode(nodeID, minerAddress string) (output string) { // TODO - allow for bootstrap Addresses as extra params (no flag) or with a flagh but allow for multiple addresses

	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
			output = fmt.Sprintf("%v", err)
		}
	}()

	fmt.Printf("Starting Node %s\n", nodeID)

	if len(minerAddress) > 0 {
		if wallet.ValidateAddress(minerAddress) {
			fmt.Println("Mining is on. Address to receive rewards: ", minerAddress)
		} else {
			log.Panic("Wrong miner address!")
		}
	}
	// network.StartServer(nodeID, minerAddress)

	host, err := network.ExternalIP()
	network.HandleError(err)
	address := ""
	port, err := strconv.Atoi(nodeID)
	network.HandleError(err)
	bootstrapAddresses := []string{}
	// FIXME - temp server node .. always connected .. needed for other to join the network. (bootstrap)
	if nodeID != "4000" {
		bootstrapAddresses = []string{"[2a02:8084:a5bf:f680:1cfd:d24c:82aa:834]:2000"} //[]string{"[2a02:8084:a5bf:f680:1cfd:d24c:82aa:834]:4000"}
	}
	network.StartServer(host, uint16(port), address, minerAddress, bootstrapAddresses)

	return "Success!"
}

func ReindexUTXO(nodeID string) (output string) {

	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
			output = fmt.Sprintf("%v", err)
		}
	}()

	chain := blockchain.ContinueBlockChain(nodeID)
	defer chain.Database.Close()
	UTXOSet := blockchain.UTXOSet{chain}
	UTXOSet.Reindex()

	count := UTXOSet.CountTransactions()
	return ("Done! There are " + strconv.Itoa(count) + " transactions in the UTXO set.")
}

func ListAddresses(nodeID string) (output string) {

	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
			output = fmt.Sprintf("%v", err)
		}
	}()

	wallets, _ := wallet.CreateWallets(nodeID)
	addresses := wallets.GetAllAddresses()

	result := " "

	for _, address := range addresses {
		result += (address + "\n")
	}

	return result
}

func CreateWallet(nodeID string) (output string) {

	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
			output = fmt.Sprintf("%v", err)
		}
	}()

	wallets, _ := wallet.CreateWallets(nodeID)
	address := wallets.AddWallet()
	wallets.SaveFile(nodeID)

	return address
}

func PrintChain(nodeID string) (output string) {

	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
			output = fmt.Sprintf("%v", err)
		}
	}()

	chain := blockchain.ContinueBlockChain(nodeID)
	defer chain.Database.Close()
	iter := chain.Iterator()

	result := ""

	for {
		block := iter.Next()

		result += ("Hash: " + string(block.Hash) + "\n")
		result += ("Prev. hash: " + string(block.PrevHash) + "\n")
		pow := blockchain.NewProof(block)
		result += ("PoW: " + strconv.FormatBool(pow.Validate()) + "\n")
		for _, tx := range block.Transactions {
			result += (tx.String() + "\n")
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}

	return result
}

func CreateBlockChain(address, nodeID string) (output string) {

	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
			output = fmt.Sprintf("%v", err)
		}
	}()

	if !wallet.ValidateAddress(address) {
		return ("Address is not Valid")
	}
	chain := blockchain.InitBlockChain(address, nodeID)
	defer chain.Database.Close()

	UTXOSet := blockchain.UTXOSet{chain}
	UTXOSet.Reindex()

	return ("Finished!")
}

func GetBalance(address, nodeID string) (output string) {

	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
			output = fmt.Sprintf("%v", err)
		}
	}()

	if !wallet.ValidateAddress(address) {
		return ("Address is not Valid")
	}
	chain := blockchain.ContinueBlockChain(nodeID)
	UTXOSet := blockchain.UTXOSet{chain}
	defer chain.Database.Close()

	balance := 0
	pubKeyHash := wallet.Base58Decode([]byte(address))
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]
	UTXOs := UTXOSet.FindUnspentTransactions(pubKeyHash)

	for _, out := range UTXOs {
		balance += out.Value
	}

	return strconv.Itoa(balance)
}

func Send(from, to string, amount int, nodeID string, mineNow bool) (output string) {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
			output = fmt.Sprintf("%v", err)
		}
	}()

	if !wallet.ValidateAddress(to) {
		return ("Address is not Valid")
	}
	if !wallet.ValidateAddress(from) {
		return ("Address is not Valid")
	}
	chain := blockchain.ContinueBlockChain(nodeID)
	UTXOSet := blockchain.UTXOSet{chain}
	defer chain.Database.Close()

	wallets, err := wallet.CreateWallets(nodeID)
	if err != nil {
		log.Panic(err)
	}
	wallet := wallets.GetWallet(from)

	tx := blockchain.NewTransaction(&wallet, to, amount, &UTXOSet)
	if mineNow {
		cbTx := blockchain.CoinbaseTx(from, "")
		txs := []*blockchain.Transaction{cbTx, tx}
		block := chain.MineBlock(txs)
		UTXOSet.Update(block)
	} else {
		network.SendTx(network.Overlay.Table().Peers()[0].Address, tx) // FIXME possibly - replace Overlay.Table().Peers()[0].Address... with a mining bucket kademlia ??
		// fmt.Println("send tx")
	}

	return ("Success!")
}


func StartNodeStream(nodeID, minerAddress string) (output string)  { // TODO - allow for bootstrap Addresses as extra params (no flag) or with a flagh but allow for multiple addresses

	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
			output = fmt.Sprintf("%v", err)
		}
	}()

	fmt.Printf("Starting Node %s\n", nodeID)

	if len(minerAddress) > 0 {
		if wallet.ValidateAddress(minerAddress) {
			fmt.Println("Mining is on. Address to receive rewards: ", minerAddress)
		} else {
			log.Panic("Wrong miner address!")
		}
	}
	// network.StartServer(nodeID, minerAddress)

	host, err := network.ExternalIP()
	network.HandleError(err)
	address := ""
	port, err := strconv.Atoi(nodeID)
	network.HandleError(err)
	bootstrapAddresses := []string{}
	// FIXME - temp server node .. always connected .. needed for other to join the network. (bootstrap)
	if nodeID != "4000" {
		bootstrapAddresses = []string{"[2a02:8084:a5bf:f680:1cfd:d24c:82aa:834]:2000"} //[]string{"[2a02:8084:a5bf:f680:1cfd:d24c:82aa:834]:4000"}
	}
	network.StartServer(host, uint16(port), address, minerAddress, bootstrapAddresses)

	return
}