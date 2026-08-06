package query

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed prod_service_traffic_no_ib_ob.json
var _prodServiceTrafficNoIBOB string

func TestDecodeProdServiceTrafficNoIBOB(t *testing.T) {
	t.Parallel()

	var records []ProdServiceTrafficRecord
	err := json.Unmarshal([]byte(_prodServiceTrafficNoIBOB), &records)
	require.NoError(t, err)
	require.NotEmpty(t, records)
}
