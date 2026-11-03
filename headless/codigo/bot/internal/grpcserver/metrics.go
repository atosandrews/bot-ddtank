package grpcserver

import (
	"context"
	"errors"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/metrics"
	"ddbot/bot/internal/session"
)

const maxMetricsSessionIDBytes = 128

func (s *Server) GetSessionMetrics(ctx context.Context, request *pb.GetSessionMetricsRequest) (*pb.GetSessionMetricsResponse, error) {
	tenant, err := s.authenticate(ctx)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}
	if request.GetBotSessionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "bot_session_id is required")
	}
	if len(request.BotSessionId) > maxMetricsSessionIDBytes {
		return nil, status.Error(codes.InvalidArgument, "bot_session_id exceeds 128 bytes")
	}
	if strings.TrimSpace(request.BotSessionId) != request.BotSessionId {
		return nil, status.Error(codes.InvalidArgument, "bot_session_id contains surrounding whitespace")
	}

	snapshot, attached, err := s.registry.Metrics(tenant, request.BotSessionId)
	if err != nil {
		var domain *session.Error
		if errors.As(err, &domain) {
			switch domain.Code {
			case "session_not_found":
				return nil, status.Error(codes.NotFound, "session not found")
			case "service_closed":
				return nil, status.Error(codes.Unavailable, "bot is stopping")
			default:
				return nil, status.Error(codes.Internal, "session metrics unavailable")
			}
		}

		return nil, status.Error(codes.Internal, "session metrics unavailable")
	}

	now := time.Now()
	response := &pb.GetSessionMetricsResponse{
		BotSessionId:       request.BotSessionId,
		ConnectionState:    pb.MetricsConnectionState_METRICS_CONNECTION_STATE_DETACHED,
		StartedAtUnixMilli: snapshot.StartedAt.UnixMilli(),
		ObservedSequence:   snapshot.ObservedSequence,
		ServerPackets:      snapshot.ServerPackets,
		ClientPackets:      snapshot.ClientPackets,
		IncompletePackets:  snapshot.IncompletePackets,
		Totals:             combatMetrics(snapshot.Totals, now),
	}
	if attached {
		response.ConnectionState = pb.MetricsConnectionState_METRICS_CONNECTION_STATE_ATTACHED
	}
	if !snapshot.ObservedAt.IsZero() {
		response.ObservedAtUnixMilli = snapshot.ObservedAt.UnixMilli()
	}

	for _, instance := range snapshot.Instances {
		grouping, err := instanceGrouping(instance.Key.Grouping)
		if err != nil {
			return nil, err
		}
		response.Instances = append(response.Instances, &pb.InstanceMetrics{
			Grouping:   grouping,
			RoomType:   instance.Key.RoomType,
			InstanceId: instance.Key.InstanceID,
			Difficulty: instance.Key.Difficulty,
			Totals:     combatMetrics(instance.Totals, now),
		})
	}

	return response, nil
}

func activityMetrics(activity metrics.Activity, now time.Time) *pb.ActivityMetrics {
	result := &pb.ActivityMetrics{
		Started:                activity.Started,
		Completed:              activity.Completed,
		Won:                    activity.Won,
		Lost:                   activity.Lost,
		UnknownOutcomes:        activity.UnknownOutcomes,
		Abandoned:              activity.Abandoned,
		CompletedDurationMilli: activity.CompletedDuration.Milliseconds(),
		Active:                 !activity.ActiveSince.IsZero(),
	}
	if result.Active {
		result.ActiveDurationMilli = now.Sub(activity.ActiveSince).Milliseconds()
	}

	return result
}

func combatMetrics(totals metrics.Totals, now time.Time) *pb.CombatMetrics {
	result := &pb.CombatMetrics{
		Fights:                 activityMetrics(totals.Fights, now),
		Missions:               activityMetrics(totals.Missions, now),
		PlayerTurns:            totals.PlayerTurns,
		PlayerShotsResolved:    totals.PlayerShotsResolved,
		PlayerPropsConfirmed:   totals.PlayerPropsConfirmed,
		MissionExperience:      totals.MissionExperience,
		FightExperience:        totals.FightExperience,
		AwardsReceived:         totals.AwardsReceived,
		RewardItemsReceived:    totals.RewardItemsReceived,
		UngroupedAwards:        totals.UngroupedAwards,
		UngroupedRewardItems:   totals.UngroupedRewardItems,
		UnreadableFightResults: totals.UnreadableFightResults,
	}
	for _, reward := range totals.Rewards {
		result.Rewards = append(result.Rewards, &pb.RewardMetrics{
			TemplateId: reward.TemplateID,
			Quantity:   reward.Quantity,
			Awards:     reward.Awards,
		})
	}
	return result
}

func instanceGrouping(grouping metrics.InstanceGrouping) (pb.InstanceMetricsGrouping, error) {
	switch grouping {
	case metrics.InstanceGroupingKnown:
		return pb.InstanceMetricsGrouping_INSTANCE_METRICS_GROUPING_KNOWN, nil
	case metrics.InstanceGroupingUnknown:
		return pb.InstanceMetricsGrouping_INSTANCE_METRICS_GROUPING_UNKNOWN_IDENTITY, nil
	case metrics.InstanceGroupingOverflow:
		return pb.InstanceMetricsGrouping_INSTANCE_METRICS_GROUPING_CAPACITY_OVERFLOW, nil
	default:
		return pb.InstanceMetricsGrouping_INSTANCE_METRICS_GROUPING_UNSPECIFIED, status.Errorf(codes.Internal, "unknown instance grouping %d", grouping)
	}
}
