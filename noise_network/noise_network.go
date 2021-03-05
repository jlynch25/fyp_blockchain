package noisenetwork

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jlynch25/golang-blockchain/blockchain"
	"github.com/perlin-network/noise"
	"github.com/perlin-network/noise/kademlia"
	"gopkg.in/vrecan/death.v3"
)

const (
	printedLength = 8 // printedLength is the total prefix length of a public key associated to a chat users ID.
	version       = 1
	commandLength = 12
)

var (
	// Node the current user
	Node *noise.Node
	// the current blockchain
	chain *blockchain.BlockChain
	// Overlay is the main pool of peers
	Overlay *kademlia.Protocol

	minerAddress string

	blocksInTransit = [][]byte{}
	memoryPool      = make(map[string]blockchain.Transaction)
)

type commandMessage struct {
	cmdType  string
	contents []byte
}

// Addr struct
type Addr struct {
	AddrList []string
}

// Block struct
type Block struct {
	AddrFrom string
	Block    []byte
}

// GetBlocks struct
type GetBlocks struct {
	AddrFrom string
}

// GetData struct
type GetData struct {
	AddrFrom string
	Type     string
	ID       []byte
}

// Inv struct
type Inv struct {
	AddrFrom string
	Type     string
	Items    [][]byte
}

// Tx struct
type Tx struct {
	AddrFrom    string
	Transaction []byte
}

// Version struct
type Version struct {
	Version    int
	BestHeight int
	AddrFrom   string
}

// StartServer function
func StartServer(hostFlag net.IP, portFlag uint16, addressFlag, minerAddress string, bootstrapAddresses []string) {

	// Create a new configured node.
	node, err := noise.NewNode(
		noise.WithNodeBindHost(hostFlag),
		noise.WithNodeBindPort(portFlag),
		noise.WithNodeAddress(addressFlag),
	)
	HandleError(err)

	// Release resources associated to Node at the end of the program.
	defer node.Close()

	Node = node

	minerAddress = minerAddress

	chain = blockchain.ContinueBlockChain(fmt.Sprint(portFlag)) //FIXME Node.ID().Port??? //uint16 to string
	defer chain.Database.Close()
	go CloseDB(chain)

	// Register the X/Y/Z Go type to the Node with an associated unmarshal function.
	node.RegisterMessage(commandMessage{}, unmarshalCommandMessage)

	// Register a X/Y/Z handler to the Node.
	node.Handle(handle)

	// Instantiate Kademlia.
	events := kademlia.Events{
		OnPeerAdmitted: func(id noise.ID) {
			fmt.Printf("Learned about a new peer %s(%s).\n", id.Address, id.ID.String()[:printedLength])
		},
		OnPeerEvicted: func(id noise.ID) {
			fmt.Printf("Forgotten a peer %s(%s).\n", id.Address, id.ID.String()[:printedLength])
		},
	}

	Overlay = kademlia.New(kademlia.WithProtocolEvents(events))

	// Bind Kademlia to the Node.
	node.Bind(Overlay.Protocol())

	// Have the Node start listening for new peers.
	HandleError(node.Listen())

	// Print out the nodes ID and a help message comprised of commands.
	help(node)

	// Ping nodes to initially bootstrap and discover peers from.
	bootstrap(node, bootstrapAddresses...) // FIXME addressFlag????

	// Attempt to discover peers if we are bootstrapped to any nodes.
	discover(Overlay)

	//TODO - check if kademlia auto finds closest, else, use FindClosest(target noise.PublicKey, k int)
	peers := Overlay.Table().Peers()
	if len(peers) > 0 {
		// TODO - ping node to check if its accessable, if not move on to next closest peers[1]
		SendVersion(peers[0].Address, chain)
	}

	WaitForCtrlC()
	fmt.Printf("\n")

}

// RequestBlocks function
func RequestBlocks() {
	for _, id := range Overlay.Table().Peers() {
		SendGetBlocks(id.Address)
	}
}

// SendBlock function
func SendBlock(addr string, b *blockchain.Block) {
	data := Block{Node.ID().Address, b.Serialize()}
	payload := GobEncode(data)
	request := commandMessage{cmdType: "block", contents: payload}

	SendDataToOne(addr, request)
}

// SendInv function
func SendInv(address, kind string, items [][]byte) {
	inventory := Inv{Node.ID().Address, kind, items}
	payload := GobEncode(inventory)
	request := commandMessage{cmdType: "inv", contents: payload}

	SendDataToOne(address, request)
}

// SendTx function
func SendTx(addr string, tnx *blockchain.Transaction) {
	data := Tx{Node.ID().Address, tnx.Serialize()}
	payload := GobEncode(data)
	request := commandMessage{cmdType: "tx", contents: payload}

	SendDataToOne(addr, request)
}

// SendVersion function
func SendVersion(addr string, chain *blockchain.BlockChain) {
	bestHeight := chain.GetBestHeight()
	payload := GobEncode(Version{version, bestHeight, Node.ID().Address})
	request := commandMessage{cmdType: "version", contents: payload}
	SendDataToOne(addr, request)
}

// SendGetBlocks function
func SendGetBlocks(address string) {
	payload := GobEncode(GetBlocks{Node.ID().Address})
	request := commandMessage{cmdType: "getblocks", contents: payload}

	SendDataToOne(address, request)
}

// SendGetData function
func SendGetData(address, kind string, id []byte) {
	payload := GobEncode(GetData{Node.ID().Address, kind, id})
	request := commandMessage{cmdType: "getdata", contents: payload}

	SendDataToOne(address, request)
}

// SendData function send data to all ... use case ?
func SendData(addr string, msg noise.Serializable) {
	for _, id := range Overlay.Table().Peers() {
		if id.Address != addr {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			err := Node.SendMessage(ctx, id.Address, msg)
			cancel()

			if err != nil {
				fmt.Printf("Failed to send message to %s(%s). Skipping... [error: %s]\n",
					id.Address,
					id.ID.String()[:printedLength],
					err,
				)
				continue
			}
		}
	}
}

// SendDataToOne function
func SendDataToOne(addr string, msg noise.Serializable) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	err := Node.SendMessage(ctx, addr, msg)
	cancel()

	if err != nil {
		fmt.Printf("Failed to send message to %s. Skipping... [error: %s]\n",
			addr,
			err,
		)
	}
}

// handle handles valid command messages from peers.
func handle(ctx noise.HandlerContext) error {
	if ctx.IsRequest() {
		return nil
	}

	obj, err := ctx.DecodeMessage()
	if err != nil {
		return nil
	}

	msg, ok := obj.(commandMessage)
	if !ok {
		return nil
	}

	if len(msg.cmdType) == 0 || len(msg.contents) == 0 {
		return nil
	}

	fmt.Printf("Received %s command\n", msg.cmdType)

	switch msg.cmdType {
	// case "addr":
	// 	HandleAddr(cmd.contents)
	case "block":
		HandleBlock(msg.contents)
	case "inv":
		HandleInv(msg.contents)
	case "getblocks":
		HandleGetBlocks(msg.contents)
	case "getdata":
		HandleGetData(msg.contents)
	case "tx":
		HandleTx(msg.contents)
	case "version":
		HandleVersion(msg.contents)
	default:
		fmt.Println("Unknown command")
	}

	return nil
}

// HandleInv function
func HandleInv(request []byte) {
	var buff bytes.Buffer
	var payload Inv

	buff.Write(request)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	HandleError(err)

	fmt.Printf("Recevied inventory with %d %s\n", len(payload.Items), payload.Type)

	if payload.Type == "block" {
		blocksInTransit = payload.Items

		blockHash := payload.Items[0]
		SendGetData(payload.AddrFrom, "block", blockHash)

		newInTransit := [][]byte{}
		for _, b := range blocksInTransit {
			if bytes.Compare(b, blockHash) != 0 {
				newInTransit = append(newInTransit, b)
			}
		}
		blocksInTransit = newInTransit
	}

	if payload.Type == "tx" {
		txID := payload.Items[0]

		if memoryPool[hex.EncodeToString(txID)].ID == nil {
			SendGetData(payload.AddrFrom, "tx", txID)
		}
	}
}

// HandleBlock function
func HandleBlock(request []byte) {
	var buff bytes.Buffer
	var payload Block

	buff.Write(request)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	HandleError(err)

	blockData := payload.Block
	block := blockchain.Deserialize(blockData)

	fmt.Println("Recevid a new block!")
	chain.AddBlock(block)

	fmt.Printf("Added block %x\n", block.Hash)

	if len(blocksInTransit) > 0 {
		blockHash := blocksInTransit[0]
		SendGetData(payload.AddrFrom, "block", blockHash)

		blocksInTransit = blocksInTransit[1:]
	} else {
		UTXOSet := blockchain.UTXOSet{chain}
		UTXOSet.Reindex()
	}
}

// HandleGetBlocks function
func HandleGetBlocks(request []byte) {
	var buff bytes.Buffer
	var payload GetBlocks

	buff.Write(request)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	HandleError(err)

	blocks := chain.GetBlockHashes()
	SendInv(payload.AddrFrom, "block", blocks)
}

// HandleGetData function
func HandleGetData(request []byte) {
	var buff bytes.Buffer
	var payload GetData

	buff.Write(request)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	HandleError(err)

	if payload.Type == "block" {
		block, err := chain.GetBlock([]byte(payload.ID))
		if err != nil {
			return
		}

		SendBlock(payload.AddrFrom, &block)
	}

	if payload.Type == "tx" {
		txID := hex.EncodeToString(payload.ID)
		tx := memoryPool[txID]

		SendTx(payload.AddrFrom, &tx)
	}
}

// HandleTx function
func HandleTx(request []byte) {
	var buff bytes.Buffer
	var payload Tx

	buff.Write(request)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	HandleError(err)

	txData := payload.Transaction
	tx := blockchain.DeserializeTransaction(txData)
	memoryPool[hex.EncodeToString(tx.ID)] = tx

	fmt.Printf("%s, %d\n", Node.ID().Address, len(memoryPool))

	if Node.ID().Address == Overlay.Table().Peers()[0].Address { //FIXME - look into
		for _, id := range Overlay.Table().Peers() {
			if id.Address != Node.ID().Address && id.Address != payload.AddrFrom {
				SendInv(id.Address, "tx", [][]byte{tx.ID})
			}
		}
	} else {
		if len(memoryPool) >= 2 && len(minerAddress) > 0 {
			MineTx()
		}
	}
}

// MineTx function
func MineTx() {
	var txs []*blockchain.Transaction

	for id := range memoryPool {
		fmt.Printf("tx: %s\n", memoryPool[id].ID)
		tx := memoryPool[id]
		if chain.VerifyTransaction(&tx) {
			txs = append(txs, &tx)
		}
	}

	if len(txs) == 0 {
		fmt.Println("All Transactions are invalid")
		return
	}

	cbTx := blockchain.CoinbaseTx(minerAddress, "")
	txs = append(txs, cbTx)

	newBlock := chain.MineBlock(txs)
	UTXOSet := blockchain.UTXOSet{chain}
	UTXOSet.Reindex()

	fmt.Println("New BLock mined")

	for _, tx := range txs {
		txID := hex.EncodeToString(tx.ID)
		delete(memoryPool, txID)
	}

	for _, id := range Overlay.Table().Peers() {
		if id.Address != Node.ID().Address {
			SendInv(id.Address, "block", [][]byte{newBlock.Hash})
		}
	}

	if len(memoryPool) > 0 {
		MineTx()
	}
}

//HandleVersion function
func HandleVersion(request []byte) {
	var buff bytes.Buffer
	var payload Version

	buff.Write(request)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	HandleError(err)

	bestHeight := chain.GetBestHeight()
	otherHeight := payload.BestHeight

	if bestHeight < otherHeight {
		SendGetBlocks(payload.AddrFrom)
	} else if bestHeight > otherHeight {
		SendVersion(payload.AddrFrom, chain)
	}

}

func (m commandMessage) Marshal() []byte {
	return append(CmdToBytes(m.cmdType), m.contents...)
}

func unmarshalCommandMessage(buf []byte) (commandMessage, error) {
	return commandMessage{cmdType: BytesToCmd(buf[:commandLength]), contents: buf[commandLength:]}, nil
}

// help prints out the users ID and commands available.
func help(node *noise.Node) {
	fmt.Printf("Your ID is %s(%s). Type '/discover' to attempt to discover new "+
		"peers, or '/peers' to list out all peers you are connected to.\n",
		node.ID().Address,
		node.ID().ID.String()[:printedLength],
	)
}

// bootstrap pings and dials an array of network addresses which we may interact with and  discover peers from.
func bootstrap(node *noise.Node, addresses ...string) {
	fmt.Printf("Addresses: %s \n", addresses)
	for _, addr := range addresses {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_, err := node.Ping(ctx, addr)
		cancel()

		if err != nil {
			fmt.Printf("Failed to ping bootstrap node (%s). Skipping... [error: %s]\n", addr, err)
			continue
		}
	}
}

// discover uses Kademlia to discover new peers from nodes we already are aware of.
func discover(overlay *kademlia.Protocol) {
	ids := overlay.Discover()

	var str []string
	for _, id := range ids {
		str = append(str, fmt.Sprintf("%s(%s)", id.Address, id.ID.String()[:printedLength]))
	}

	if len(ids) > 0 {
		fmt.Printf("Discovered %d peer(s): [%v]\n", len(ids), strings.Join(str, ", "))
	} else {
		fmt.Printf("Did not discover any peers.\n")
	}
}

// peers prints out all peers we are already aware of.
func peers(overlay *kademlia.Protocol) {
	ids := overlay.Table().Peers()

	var str []string
	for _, id := range ids {
		str = append(str, fmt.Sprintf("%s(%s)", id.Address, id.ID.String()[:printedLength]))
	}

	fmt.Printf("You know %d peer(s): [%v]\n", len(ids), strings.Join(str, ", "))
}

// CmdToBytes function
func CmdToBytes(cmd string) []byte {
	var bytes [commandLength]byte

	for i, c := range cmd {
		bytes[i] = byte(c)
	}

	return bytes[:]
}

// BytesToCmd function
func BytesToCmd(bytes []byte) string {
	var cmd []byte

	for _, b := range bytes {
		if b != 0x0 {
			cmd = append(cmd, b)
		}
	}

	return fmt.Sprintf("%s", cmd)
}

// GobEncode function
func GobEncode(data interface{}) []byte {
	var buff bytes.Buffer

	enc := gob.NewEncoder(&buff)
	err := enc.Encode(data)
	HandleError(err)

	return buff.Bytes()
}

// CloseDB function
func CloseDB(chain *blockchain.BlockChain) {
	d := death.NewDeath(syscall.SIGINT, syscall.SIGTERM, os.Interrupt) //linux, mac, windows

	d.WaitForDeathWithFunc(func() {
		defer os.Exit(1)
		defer runtime.Goexit()
		chain.Database.Close()
	})
}

// HandleError function
func HandleError(err error) {
	if err != nil {
		log.Panic(err)
	}
}

// WaitForCtrlC function
func WaitForCtrlC() {
	var endWaiter sync.WaitGroup
	endWaiter.Add(1)
	var signalChannel chan os.Signal
	signalChannel = make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt)
	go func() {
		<-signalChannel
		endWaiter.Done()
	}()
	endWaiter.Wait()
}
