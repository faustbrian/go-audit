package postgres

import (
	"context"
	"errors"
	"testing"

	identitypostgres "github.com/faustbrian/go-identity/postgres"
)

func TestIdentitySCIMBulkSkipAuditsRequireNativeAuthority(t *testing.T) {
	c, err := NewIdentityContributor(Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name        string
		contributor *IdentityContributor
		ctx         context.Context
	}{
		{"zero Work", c, context.Background()},
		{"nil context", c, nil},
		{"nil contributor", nil, context.Background()},
		{"unconfigured contributor", &IdentityContributor{}, context.Background()},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.contributor.PersistSCIMBulkSkipAudits(test.ctx, identitypostgres.Work{}); err == nil {
				t.Fatal("cutoff persistence accepted invalid authority")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.PersistSCIMBulkSkipAudits(ctx, identitypostgres.Work{}); !errors.Is(err, context.Canceled) {
		t.Fatal("cutoff persistence lost cancellation", err)
	}
}
