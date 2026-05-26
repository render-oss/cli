package postgres

import pgclient "github.com/render-oss/cli/pkg/client/postgres"

// ModernPlans lists the plan names suggested in --plan help text. The API
// accepts additional account-specific plan names (custom plans); this list is
// for documentation only, not validation. Hand-curated subset of the
// pgclient.PostgresPlans constants — update when a new modern plan is added.
var ModernPlans = []string{
	string(pgclient.Free),
	string(pgclient.Basic256mb),
	string(pgclient.Basic1gb),
	string(pgclient.Basic4gb),
	string(pgclient.Pro4gb),
	string(pgclient.Pro8gb),
	string(pgclient.Pro16gb),
	string(pgclient.Pro32gb),
	string(pgclient.Pro64gb),
	string(pgclient.Pro128gb),
	string(pgclient.Pro192gb),
	string(pgclient.Pro256gb),
	string(pgclient.Pro384gb),
	string(pgclient.Pro512gb),
	string(pgclient.Accelerated16gb),
	string(pgclient.Accelerated32gb),
	string(pgclient.Accelerated64gb),
	string(pgclient.Accelerated128gb),
	string(pgclient.Accelerated256gb),
	string(pgclient.Accelerated384gb),
	string(pgclient.Accelerated512gb),
	string(pgclient.Accelerated768gb),
	string(pgclient.Accelerated1024gb),
}
