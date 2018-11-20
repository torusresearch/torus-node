package hbbft

// Transaction is an interface that abstract the underlying data of the actual
// transaction. This allows package hbbft to be easily adopted by other
// applications.
type Transaction interface {
	Hash() []byte
}
