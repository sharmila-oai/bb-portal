package dbcleanupservice

import (
	"context"

	"github.com/buildbarn/bb-portal/ent/gen/ent/bazelinvocation"
	"github.com/buildbarn/bb-portal/ent/gen/ent/incompletebuildlog"
	"github.com/buildbarn/bb-storage/pkg/util"
	"go.opentelemetry.io/otel/attribute"
)

const expiredIncompleteLogDeletionLimit = 128

// RemoveExpiredIncompleteLogs deletes incomplete logs for completed invocations
// once they exceed their dedicated retention window.
func (dc *DbCleanupService) RemoveExpiredIncompleteLogs(ctx context.Context) error {
	ctx, span := dc.tracer.Start(ctx, "DbCleanupService.RemoveExpiredIncompleteLogs")
	defer span.End()

	if dc.incompleteLogRetention == nil {
		return nil
	}

	cutoffTime := dc.clock.Now().UTC().Add(-*dc.incompleteLogRetention)
	invocationIDs, err := dc.db.Ent().BazelInvocation.Query().
		Where(
			bazelinvocation.BepCompleted(true),
			bazelinvocation.EndedAtLT(cutoffTime),
			bazelinvocation.HasIncompleteBuildLogs(),
		).
		Limit(expiredIncompleteLogDeletionLimit).
		IDs(ctx)
	if err != nil {
		return util.StatusWrap(err, "Failed to query invocations with expired incomplete logs")
	}
	if len(invocationIDs) == 0 {
		return nil
	}

	deletedLogs, err := dc.db.Ent().IncompleteBuildLog.Delete().
		Where(
			incompletebuildlog.HasBazelInvocationWith(
				bazelinvocation.IDIn(invocationIDs...),
			),
		).
		Exec(ctx)
	if err != nil {
		return util.StatusWrap(err, "Failed to remove expired incomplete logs")
	}

	span.SetAttributes(
		attribute.Int("invocations_scanned", len(invocationIDs)),
		attribute.Int("deleted_incomplete_logs", deletedLogs),
	)
	return nil
}
