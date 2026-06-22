package manifest

import (
	"sort"
	"time"
)

type Snapshot struct {
	Name      string    `json:"name" yaml:"name"`
	Cadence   string    `json:"cadence" yaml:"cadence"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
}

type RetentionPolicy struct {
	Daily   int
	Weekly  int
	Monthly int
}

type RetentionPlan struct {
	Keep  []Snapshot `json:"keep"`
	Prune []Snapshot `json:"prune"`
}

func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{Daily: 14, Weekly: 8, Monthly: 6}
}

func PlanRetention(snapshots []Snapshot, policy RetentionPolicy) RetentionPlan {
	byCadence := map[string][]Snapshot{}
	for _, snapshot := range snapshots {
		byCadence[snapshot.Cadence] = append(byCadence[snapshot.Cadence], snapshot)
	}

	var plan RetentionPlan
	for _, cadence := range []string{"daily", "weekly", "monthly"} {
		items := byCadence[cadence]
		sortSnapshotsNewestFirst(items)
		keepCount := keepCountForCadence(policy, cadence)
		for i, snapshot := range items {
			if i < keepCount {
				plan.Keep = append(plan.Keep, snapshot)
				continue
			}
			plan.Prune = append(plan.Prune, snapshot)
		}
	}

	sortSnapshotsNewestFirst(plan.Keep)
	sortSnapshotsNewestFirst(plan.Prune)
	return plan
}

func keepCountForCadence(policy RetentionPolicy, cadence string) int {
	switch cadence {
	case "daily":
		return policy.Daily
	case "weekly":
		return policy.Weekly
	case "monthly":
		return policy.Monthly
	default:
		return 0
	}
}

func sortSnapshotsNewestFirst(snapshots []Snapshot) {
	sort.Slice(snapshots, func(i, j int) bool {
		if !snapshots[i].CreatedAt.Equal(snapshots[j].CreatedAt) {
			return snapshots[i].CreatedAt.After(snapshots[j].CreatedAt)
		}
		return snapshots[i].Name < snapshots[j].Name
	})
}
