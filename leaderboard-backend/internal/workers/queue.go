package workers

import "context"

type Queue interface {
	Publish(ctx context.Context, event ScoreUpdateEvent) error
	Consume(ctx context.Context) (<-chan ScoreUpdateEvent, error)
}
