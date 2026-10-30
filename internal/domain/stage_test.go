package domain

import "testing"

func TestStageAndOutcome(t *testing.T) {
	stages := map[string]int{
		StatusToApply: 0, StatusSaved: 0, StatusAwaitingReview: 1, StatusApproved: 1, StatusApplied: 2,
		StatusSubmitted: 2, StatusScreening: 3, StatusInterviewing: 4, StatusFinal: 5, StatusOffer: 6,
	}
	for status, want := range stages {
		if got := StageOf(status); got != want || IsOutcome(status) {
			t.Fatalf("%s: stage %d outcome %v, want %d", status, got, IsOutcome(status), want)
		}
	}
	for _, o := range []string{StatusWaiting, StatusNeedsInput, StatusFailed, StatusRejected, StatusWithdrawn, StatusGhosted} {
		if !IsOutcome(o) {
			t.Fatalf("%s should be an outcome", o)
		}
	}
	if IsStop(StatusWaiting) || IsStop(StatusNeedsInput) || !IsStop(StatusGhosted) {
		t.Fatal("only failed, rejected, withdrawn and ghosted stop the pursuit")
	}
}
