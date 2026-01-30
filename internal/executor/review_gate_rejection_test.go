package executor

import "testing"

func TestReviewGateRejection_FailsTask(t *testing.T) {
	t.Parallel()

	got := isReviewPhaseGateRejectionFatal("review")
	if !got {
		t.Error("expected review phase gate rejection to be fatal, but got false")
	}
}

func TestNonReviewGateRejection_ContinuesExecution(t *testing.T) {
	t.Parallel()

	cases := []struct {
		phaseID string
	}{
		{"implement"},
		{"spec"},
		{"tdd_write"},
		{"docs"},
	}

	for _, tc := range cases {
		t.Run(tc.phaseID, func(t *testing.T) {
			t.Parallel()

			got := isReviewPhaseGateRejectionFatal(tc.phaseID)
			if got {
				t.Errorf("expected %s phase gate rejection to NOT be fatal, but got true", tc.phaseID)
			}
		})
	}
}
