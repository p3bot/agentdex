package catalogtest

import "testing"

// FixtureBins maps synthetic fixture agent ids to unclaimable binary names so
// host PATH tools (e.g. git-delta as "delta") cannot satisfy detection. The
// catalog-valid fixture bins and every test consumer must match this table.
var FixtureBins = map[string]string{
	"alpha-cli":   "agentdex-fixture-alpha",
	"beta-tool":   "agentdex-fixture-beta",
	"gamma-agent": "agentdex-fixture-gamma",
	"delta-agent": "agentdex-fixture-delta",
}

// FixtureBin returns the fixture binary name for agentID, or fails the test.
func FixtureBin(t *testing.T, agentID string) string {
	t.Helper()
	name, ok := FixtureBins[agentID]
	if !ok {
		t.Fatalf("catalogtest: unknown fixture agent %q", agentID)
	}
	return name
}
