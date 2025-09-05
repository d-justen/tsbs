package serialize

import (
	"github.com/timescale/tsbs/pkg/data"
	"github.com/timescale/tsbs/pkg/data/usecases/common"

	"io"
)

// PointSerializer serializes a Point for writing
type PointSerializer interface {
	Serialize(p *data.Point, w io.Writer) error
}

type ConfigurableSerializer interface {
	PointSerializer
	Config(*common.DataGeneratorConfig, io.Writer) error
}
