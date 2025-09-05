package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/blagojts/viper"
	"github.com/marcboeker/go-duckdb/v2"
	"github.com/spf13/pflag"
	"github.com/timescale/tsbs/internal/utils"
	"github.com/timescale/tsbs/pkg/query"
)

// Program option vars:
var (
	showExplain bool
)

// Global vars:
var (
	runner *query.BenchmarkRunner
)

// Parse args:
func init() {
	var config query.BenchmarkRunnerConfig
	config.AddToFlagSet(pflag.CommandLine)
	pflag.Bool("show-explain", false, "Print out the EXPLAIN output for sample query")
	pflag.Parse()

	err := utils.SetupConfigFile()

	if err != nil {
		panic(fmt.Errorf("fatal error config file: %s", err))
	}

	if err := viper.Unmarshal(&config); err != nil {
		panic(fmt.Errorf("unable to decode config: %s", err))
	}

	showExplain = viper.GetBool("show-explain")

	runner = query.NewBenchmarkRunner(config)
	runner.Workers = 1

	if showExplain {
		runner.SetLimit(1)
	}
}

func main() {
	runner.Run(&query.DuckDBPool, newProcessor)
}

func prettyPrintResponse(rows *sql.Rows, q *query.DuckDB) {
	resp := make(map[string]interface{})
	resp["query"] = string(q.SqlQuery)

	var results []*interface{}
	for rows.Next() {
		var r *interface{}
		if err := rows.Scan(r); err != nil {
			panic(err)
		}
		results = append(results, r)
	}
	resp["results"] = results

	line, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		panic(err)
	}

	fmt.Println(string(line) + "\n")
}

type queryExecutorOptions struct {
	showExplain   bool
	debug         bool
	printResponse bool
}

// query.Processor interface implementation
type processor struct {
	db   *sql.DB
	opts *queryExecutorOptions
}

// query.Processor interface implementation
func newProcessor() query.Processor {
	return &processor{}
}

// query.Processor interface implementation
func (p *processor) Init(workerNumber int) {
	connector, err := duckdb.NewConnector("tsbs.duckdb", nil)
	if err != nil {
		panic(err)
	}
	p.db = sql.OpenDB(connector)
	p.opts = &queryExecutorOptions{
		showExplain:   showExplain,
		debug:         runner.DebugLevel() > 0,
		printResponse: runner.DoPrintResponses(),
	}
}

// query.Processor interface implementation
func (p *processor) ProcessQuery(q query.Query, isWarm bool) ([]*query.Stat, error) {
	// No need to run again for EXPLAIN
	if isWarm && p.opts.showExplain {
		return nil, nil
	}

	// Ensure ClickHouse query
	chQuery := q.(*query.DuckDB)

	start := time.Now()

	// SqlQuery is []byte, so cast is needed
	sqlStr := string(chQuery.SqlQuery)

	// Main action - run the query
	rows, err := p.db.Query(sqlStr)
	if err != nil {
		return nil, err
	}

	// Print some extra info if needed
	if p.opts.debug {
		fmt.Println(sqlStr)
	}
	if p.opts.printResponse {
		prettyPrintResponse(rows, chQuery)
	}

	// Finalize the query
	rows.Close()
	took := float64(time.Since(start).Nanoseconds()) / 1e6

	stat := query.GetStat()
	stat.Init(q.HumanLabelName(), took)

	return []*query.Stat{stat}, err
}
