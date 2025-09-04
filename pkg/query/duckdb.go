package query

import (
	"fmt"
	"sync"
)

// DuckDb encodes a DuckDB query.
// This will be serialized for use by the tsbs_run_queries_clickhouse program.
type DuckDB struct {
	HumanLabel       []byte
	HumanDescription []byte

	Table    []byte // e.g. "cpu"
	SqlQuery []byte
	id       uint64
}

// DuckDBPool is a sync.Pool of DuckDB Query types
var DuckDBPool = sync.Pool{
	New: func() interface{} {
		return &DuckDB{
			HumanLabel:       make([]byte, 0, 1024),
			HumanDescription: make([]byte, 0, 1024),
			Table:            make([]byte, 0, 1024),
			SqlQuery:         make([]byte, 0, 1024),
		}
	},
}

// NewDuckDB returns a new DuckDB Query instance
func NewDuckDB() *DuckDB {
	return DuckDBPool.Get().(*DuckDB)
}

// GetID returns the ID of this Query
func (ch *DuckDB) GetID() uint64 {
	return ch.id
}

// SetID sets the ID for this Query
func (ch *DuckDB) SetID(n uint64) {
	ch.id = n
}

// String produces a debug-ready description of a Query.
func (ch *DuckDB) String() string {
	return fmt.Sprintf("HumanLabel: %s, HumanDescription: %s, Table: %s, Query: %s", ch.HumanLabel, ch.HumanDescription, ch.Table, ch.SqlQuery)
}

// HumanLabelName returns the human readable name of this Query
func (ch *DuckDB) HumanLabelName() []byte {
	return ch.HumanLabel
}

// HumanDescriptionName returns the human readable description of this Query
func (ch *DuckDB) HumanDescriptionName() []byte {
	return ch.HumanDescription
}

// Release resets and returns this Query to its pool
func (ch *DuckDB) Release() {
	ch.HumanLabel = ch.HumanLabel[:0]
	ch.HumanDescription = ch.HumanDescription[:0]

	ch.Table = ch.Table[:0]
	ch.SqlQuery = ch.SqlQuery[:0]

	DuckDBPool.Put(ch)
}
