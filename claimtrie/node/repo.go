package node

// Repo defines APIs for Node to access persistence layer.
type Repo interface {
	// AppendChanges saves changes into the repo.
	// The changes can belong to different nodes, but the chronological
	// order must be preserved for the same node.
	AppendChanges(changes []Change) error

	// LoadChanges loads changes of a node up to (includes) the specified height.
	// If no changes found, both returned slice and error will be nil.
	LoadChanges(name []byte) ([]Change, error)

	DropChanges(name []byte, finalHeight int32) error

	// Close closes the repo.
	Close() error

	// IterateChildren returns change sets for each of name.+
	// Return false on f to stop the iteration.
	IterateChildren(name []byte, f func(changes []Change) bool)

	// IterateAll iterates keys until the predicate function returns false
	IterateAll(predicate func(name []byte) bool)
}
