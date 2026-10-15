package domain

import "context"

// InterestRepository stores marketing-site notification consent separately from
// product accounts, so an interest signup can never grant product access.
type InterestRepository interface {
	Subscribe(ctx context.Context, email string) error
}
