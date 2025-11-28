package idgen

import (
	"sync"

	"github.com/bwmarrin/snowflake"
)

var (
	node *snowflake.Node
	once sync.Once
)

// Initialize sets up the Snowflake ID generator with a node ID
func Initialize(nodeID int64) error {
	var err error
	once.Do(func() {
		node, err = snowflake.NewNode(nodeID)
	})
	return err
}

// GenerateID generates a new Snowflake ID as a string
func GenerateID() string {
	if node == nil {
		// Initialize with default node ID if not already initialized
		_ = Initialize(1)
	}
	return node.Generate().String()
}
