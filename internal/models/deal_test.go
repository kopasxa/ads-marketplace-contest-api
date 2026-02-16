package models

import "testing"

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		from     string
		to       string
		expected bool
	}{
		// Happy path
		{DealStatusDraft, DealStatusSubmitted, true},
		{DealStatusSubmitted, DealStatusAccepted, true},
		{DealStatusSubmitted, DealStatusRejected, true},
		{DealStatusAccepted, DealStatusAwaitingPayment, true},
		{DealStatusAwaitingPayment, DealStatusFunded, true},
		{DealStatusFunded, DealStatusCreativePending, true},
		{DealStatusCreativePending, DealStatusCreativeSubmitted, true},
		{DealStatusCreativeSubmitted, DealStatusCreativeApproved, true},
		{DealStatusCreativeSubmitted, DealStatusCreativeChangesRequested, true},
		{DealStatusCreativeChangesRequested, DealStatusCreativeSubmitted, true},
		{DealStatusCreativeApproved, DealStatusScheduled, true},
		{DealStatusCreativeApproved, DealStatusPosted, true},
		{DealStatusScheduled, DealStatusPosted, true},
		{DealStatusPosted, DealStatusHoldVerification, true},
		{DealStatusHoldVerification, DealStatusCompleted, true},
		{DealStatusHoldVerification, DealStatusHoldVerificationFailed, true},
		{DealStatusHoldVerificationFailed, DealStatusRefunded, true},

		// Cancellation paths
		{DealStatusDraft, DealStatusCancelled, true},
		{DealStatusSubmitted, DealStatusCancelled, true},
		{DealStatusAccepted, DealStatusCancelled, true},
		{DealStatusAwaitingPayment, DealStatusCancelled, true},
		{DealStatusFunded, DealStatusCancelled, true},
		{DealStatusCreativePending, DealStatusCancelled, true},
		{DealStatusScheduled, DealStatusCancelled, true},
		{DealStatusCancelled, DealStatusRefunded, true},

		// Invalid transitions
		{DealStatusDraft, DealStatusFunded, false},
		{DealStatusRejected, DealStatusAccepted, false},
		{DealStatusCompleted, DealStatusRefunded, false},
		{DealStatusRefunded, DealStatusCompleted, false},
		{DealStatusPosted, DealStatusCancelled, false},
		{DealStatusHoldVerification, DealStatusCancelled, false},
		{DealStatusCompleted, DealStatusCancelled, false},
		{DealStatusDraft, DealStatusPosted, false},
		{"nonexistent", DealStatusSubmitted, false},
		{DealStatusDraft, "nonexistent", false},

		// Creative loop
		{DealStatusCreativeChangesRequested, DealStatusCancelled, true},
		{DealStatusCreativeSubmitted, DealStatusCreativePending, false},
	}

	for _, tt := range tests {
		t.Run(tt.from+"->"+tt.to, func(t *testing.T) {
			result := IsValidTransition(tt.from, tt.to)
			if result != tt.expected {
				t.Errorf("IsValidTransition(%q, %q) = %v, want %v", tt.from, tt.to, result, tt.expected)
			}
		})
	}
}

func TestAllStatusesHaveTransitionEntry(t *testing.T) {
	allStatuses := []string{
		DealStatusDraft, DealStatusSubmitted, DealStatusRejected, DealStatusAccepted,
		DealStatusAwaitingPayment, DealStatusFunded,
		DealStatusCreativePending, DealStatusCreativeSubmitted,
		DealStatusCreativeChangesRequested, DealStatusCreativeApproved,
		DealStatusScheduled, DealStatusPosted,
		DealStatusHoldVerification, DealStatusHoldVerificationFailed,
		DealStatusCompleted, DealStatusRefunded, DealStatusCancelled,
	}

	for _, status := range allStatuses {
		if _, ok := ValidDealTransitions[status]; !ok {
			t.Errorf("status %q missing from ValidDealTransitions map", status)
		}
	}
}

func TestTerminalStatusesHaveNoTransitions(t *testing.T) {
	terminal := []string{DealStatusRejected, DealStatusCompleted, DealStatusRefunded}
	for _, status := range terminal {
		transitions := ValidDealTransitions[status]
		if len(transitions) != 0 {
			t.Errorf("terminal status %q should have no transitions, got %v", status, transitions)
		}
	}
}
