package blockchain

// TxOutput struct
type TxOutput struct {
	Value  int
	PubKey string
}

// TxInput struct
type TxInput struct {
	ID  []byte
	Out int
	Sig string
}

// CanUnlock function
func (in *TxInput) CanUnlock(data string) bool {
	return in.Sig == data
}

// CanBeUnlocked function
func (out *TxOutput) CanBeUnlocked(data string) bool {
	return out.PubKey == data
}
