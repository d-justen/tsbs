package duckdb

import (
	"context"
	"database/sql/driver"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/marcboeker/go-duckdb/v2"
	"github.com/timescale/tsbs/pkg/targets"
)

const (
	numExtraCols = 2 // one for tag_id
)

type syncCSI struct {
	m     map[string]int64
	mutex *sync.RWMutex
}

func newSyncCSI() *syncCSI {
	return &syncCSI{
		m:     make(map[string]int64),
		mutex: &sync.RWMutex{},
	}
}

// load.Processor interface implementation
type processor struct {
	connector *duckdb.Connector
	conn      driver.Conn
	conf      DuckDBConfig
	_csi      *syncCSI
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
		if err != nil {
			panic(err)
		}
		p._csi = newSyncCSI()
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
			metricCnt += p.processCSI(tableName, rows)
		}
	}
	batches.m = map[string][]*insertData{}
	batches.cnt = 0

	return metricCnt, uint64(rowCnt)
}

func (p *processor) processCSI(tableName string, rows []*insertData) uint64 {
	colLen := len(tableCols[tableName]) + numExtraCols
	tagRows, dataRows, numMetrics := p.splitTagsAndMetrics(rows, colLen)

	// Check if any of these tags has yet to be inserted
	newTags := make([][]string, 0, len(rows))
	p._csi.mutex.RLock()
	for _, cols := range tagRows {
		if _, ok := p._csi.m[cols[0]]; !ok {
			newTags = append(newTags, cols)
		}
	}
	p._csi.mutex.RUnlock()
	if len(newTags) > 0 {
		p._csi.mutex.Lock()
		tagIdxOffset := int64(len(p._csi.m))
		res := p.insertTags(newTags, tagIdxOffset)
		for k, v := range res {
			p._csi.m[k] = v
		}
		p._csi.mutex.Unlock()
	}

	p._csi.mutex.RLock()
	for i := range dataRows {
		tagKey := tagRows[i][0]
		dataRows[i][1] = p._csi.m[tagKey]
	}
	p._csi.mutex.RUnlock()

	//cols := make([]string, 0, colLen)
	//cols = append(cols, "time", "tags_id")
	//cols = append(cols, tableCols[tableName]...)

	appender, _ := duckdb.NewAppenderFromConn(p.conn, "", tableName)
	defer appender.Close()

	for _, row := range dataRows {
		vals := make([]driver.Value, len(row))
		for i := range row {
			vals[i] = row[i]
		}
		if err := appender.AppendRow(vals...); err != nil {
			panic(err)
		}
	}

	return numMetrics
}

// splitTagsAndMetrics takes an array of insertData (sharded by hypertable) and
// divides the tags from data into appropriate slices that can then be used in
// SQL queries to insert into their respective tables. Additionally, it also
// returns the number of metrics (i.e., non-tag fields) for the data processed.
func (p *processor) splitTagsAndMetrics(rows []*insertData, dataCols int) ([][]string, [][]interface{}, uint64) {
	tagRows := make([][]string, 0, len(rows))
	dataRows := make([][]interface{}, 0, len(rows))
	numMetrics := uint64(0)
	commonTagsLen := len(tableCols[tagsKey])

	for _, data := range rows {
		// Split the tags into individual common tags and an extra bit leftover
		// for non-common tags that need to be added separately. For each of
		// the common tags, remove everything before = in the form <label>=<val>
		// since we won't need it.
		tags := strings.SplitN(data.tags, ",", commonTagsLen+1)
		for i := 0; i < commonTagsLen; i++ {
			tags[i] = strings.Split(tags[i], "=")[1]
		}

		metrics := strings.Split(data.fields, ",")
		numMetrics += uint64(len(metrics) - 1) // 1 field is timestamp

		timeInt, err := strconv.ParseInt(metrics[0], 10, 64)
		if err != nil {
			panic(err)
		}
		ts := time.Unix(0, timeInt)

		// use nil at 2nd position as placeholder for tagKey
		r := make([]interface{}, 2, dataCols)
		r[0], r[1] = ts, nil

		for _, v := range metrics[1:] {
			if v == "" {
				r = append(r, nil)
				continue
			}

			num, err := strconv.ParseFloat(v, 64)
			if err != nil {
				panic(err)
			}

			r = append(r, num)
		}

		dataRows = append(dataRows, r)
		tagRows = append(tagRows, tags[:commonTagsLen])
	}

	return tagRows, dataRows, numMetrics
}

func (p *processor) insertTags(tagRows [][]string, tagIdxOffset int64) map[string]int64 {
	//tagCols := tableCols[tagsKey]
	//cols := tagCols
	//values := make([]string, 0)

	appender, err := duckdb.NewAppenderFromConn(p.conn, "", "tags")
	if err != nil {
		panic(err)
	}
	defer appender.Close()

	ret := make(map[string]int64, len(tagRows))

	for idx, value := range tagRows {
		if _, ok := ret[value[0]]; !ok {
			vals := make([]driver.Value, len(value)+1)
			id := tagIdxOffset + int64(idx)
			vals[0] = id
			for i := 0; i < len(value); i++ {
				vals[i+1] = value[i]
			}

			if err := appender.AppendRow(vals...); err != nil {
				panic(err)
			}

			ret[value[0]] = id
		}
	}

	return ret
}
