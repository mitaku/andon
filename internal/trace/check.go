package trace

import (
	"encoding/json"
	"fmt"
	"sync"

	"andon"
	"andon/internal/contract"
)

// breakdownChecker は schema/outcome.schema.json の breakdown だけを確かめる検査器（埋め込んだ schema から 1 度だけ組む）。
var breakdownChecker = sync.OnceValues(func() (*contract.Checker, error) {
	b, err := andon.Schemas.ReadFile(contract.SchemaOutcome.File())
	if err != nil {
		return nil, err
	}
	var o struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Defs       json.RawMessage            `json:"$defs"`
	}
	if err := json.Unmarshal(b, &o); err != nil || o.Properties["breakdown"] == nil {
		return nil, fmt.Errorf("%s に breakdown が無い", contract.SchemaOutcome.File())
	}
	sub := `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "required": ["breakdown"],` +
		`"properties": {"breakdown": ` + string(o.Properties["breakdown"]) + `}, "$defs": ` + string(o.Defs) + `}`
	return contract.Compile(contract.SchemaOutcome.File()+" の breakdown", []byte(sub))
})

// CheckBreakdown は outcome.json の breakdown の値 raw が schema の形かを確かめる（null も通る）。
// dispatcher は schema を通して書くが、読み手（andon metrics show・andon retro collect）は、手で直した記録や前のバージョンの記録を
// 黙って 0 に数えないために、読むときにも確かめる。
func CheckBreakdown(raw json.RawMessage) error {
	c, err := breakdownChecker()
	if err != nil {
		return err
	}
	return c.Check([]byte(`{"breakdown": ` + string(raw) + `}`))
}
