package duckdb

import (
	"context"
	"database/sql/driver"

	"github.com/marcboeker/go-duckdb/v2"
	"github.com/timescale/tsbs/pkg/targets"
	"github.com/timescale/tsbs/pkg/targets/clickhouse"
)

// load.Processor interface implementation
type processor struct {
	connector *duckdb.Connector
	conn      driver.Conn
	conf      DuckDBConfig
}

// load.Processor interface implementation
func (p *processor) Init(workerNum int, doLoad, hashWorkers bool) {
	if doLoad {
		connector, err := duckdb.NewConnector("tsbs.duckdb", nil)
		if err != nil {
			panic(err)
		}
		p.connector = connector
		p.conn, err = p.connector.Connect(context.Background())
	}
}

// load.ProcessorCloser interface implementation
func (p *processor) Close(doLoad bool) {
	p.conn.Close()
	p.connector.Close()
}

// load.Processor interface implementation
func (p *processor) ProcessBatch(b targets.Batch, doLoad bool) (uint64, uint64) {
	batches := b.(*tableArr)
	rowCnt := 0
	metricCnt := uint64(0)
	for tableName, rows := range batches.m {
		rowCnt += len(rows)
		if doLoad {
			// Do something
			panic(tableName)
		}
	}
	batches.m = map[string][]*clickhouse.InsertData{}
	batches.cnt = 0

	return metricCnt, uint64(rowCnt)
}
