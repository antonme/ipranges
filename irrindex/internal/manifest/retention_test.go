package manifest

import (
	"fmt"
	"testing"
	"time"
)

func TestRetentionPlanKeepsConfiguredWindows(t *testing.T) {
	base := time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC)
	var snapshots []Snapshot
	for i := 0; i < 20; i++ {
		snapshots = append(snapshots, Snapshot{
			Name:      fmt.Sprintf("daily-%02d", i),
			Cadence:   "daily",
			CreatedAt: base.AddDate(0, 0, -i),
		})
	}
	for i := 0; i < 10; i++ {
		snapshots = append(snapshots, Snapshot{
			Name:      fmt.Sprintf("weekly-%02d", i),
			Cadence:   "weekly",
			CreatedAt: base.AddDate(0, 0, -7*i),
		})
	}
	for i := 0; i < 8; i++ {
		snapshots = append(snapshots, Snapshot{
			Name:      fmt.Sprintf("monthly-%02d", i),
			Cadence:   "monthly",
			CreatedAt: base.AddDate(0, -i, 0),
		})
	}

	plan := PlanRetention(snapshots, DefaultRetentionPolicy())

	if got := countCadence(plan.Keep, "daily"); got != 14 {
		t.Fatalf("daily keep count = %d, want 14", got)
	}
	if got := countCadence(plan.Keep, "weekly"); got != 8 {
		t.Fatalf("weekly keep count = %d, want 8", got)
	}
	if got := countCadence(plan.Keep, "monthly"); got != 6 {
		t.Fatalf("monthly keep count = %d, want 6", got)
	}
	if got := len(plan.Prune); got != 10 {
		t.Fatalf("prune count = %d, want 10", got)
	}
	if plan.Keep[0].Name != "daily-00" {
		t.Fatalf("first kept snapshot = %s, want newest daily", plan.Keep[0].Name)
	}
}

func countCadence(snapshots []Snapshot, cadence string) int {
	count := 0
	for _, snapshot := range snapshots {
		if snapshot.Cadence == cadence {
			count++
		}
	}
	return count
}
