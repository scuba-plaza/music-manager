package web

import (
	"context"
	"testing"
	"time"
)

func jobEnded(t *testing.T, j *jobs, id string) {
	t.Helper()
	for i := 0; i < 400; i++ {
		if v, ok := j.get(id); ok && v.State != jobRunning {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %s never ended", id)
}

func TestTheOldestFinishedJobsAreTheOnesForgotten(t *testing.T) {
	j := newJobs(newHub())
	var ids []string
	for i := 0; i < maxFinished+5; i++ {
		v, err := j.start("work", func(context.Context, func(any)) (any, error) { return nil, nil })
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, v.ID)
		jobEnded(t, j, v.ID)
	}
	for i, id := range ids {
		_, kept := j.get(id)
		if forgotten := i < 4; kept == forgotten {
			t.Errorf("job %d of %d: kept = %v, want %v", i+1, len(ids), kept, !forgotten)
		}
	}
}
