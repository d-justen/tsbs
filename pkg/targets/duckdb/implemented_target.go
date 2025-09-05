package duckdb

import (
	"github.com/blagojts/viper"
	"github.com/spf13/pflag"
	"github.com/timescale/tsbs/pkg/data/serialize"
	"github.com/timescale/tsbs/pkg/data/source"
	"github.com/timescale/tsbs/pkg/targets"
	"github.com/timescale/tsbs/pkg/targets/constants"
	"github.com/timescale/tsbs/pkg/targets/timescaledb"
)

func NewTarget() targets.ImplementedTarget {
	return &duckdbTarget{}
}

type duckdbTarget struct {
}

func (t *duckdbTarget) TargetName() string {
	return constants.FormatDuckDB
}

func (t *duckdbTarget) Serializer() serialize.PointSerializer {
	return &timescaledb.Serializer{}
}

func (t *duckdbTarget) Benchmark(
	targetDB string, dataSourceConfig *source.DataSourceConfig, v *viper.Viper,
) (targets.Benchmark, error) {
	panic("not implemented")
}

func (t *duckdbTarget) TargetSpecificFlags(flagPrefix string, flagSet *pflag.FlagSet) {
}
