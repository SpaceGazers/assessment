// Network protocol and other communications issues.
package comm

type ForwardBlock struct {
	BlockId string
	Nodes []string
}

type HaveBlock struct {
	BlockId string
	NodeId string
}