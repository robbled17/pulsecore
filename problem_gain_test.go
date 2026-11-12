package main

import "testing"

func TestAssignCascadedGains_SumLEQRoom(t *testing.T) {
	cases := []struct {
		score int
		raw   []int
	}{
		{84, []int{12, 10}},
		{84, []int{12, 10, 8}},
		{70, []int{20, 20, 20}},
		{90, []int{16, 12}},
		{99, []int{12, 10}},
		{50, []int{30, 30, 30}},
		{0, []int{40, 40, 40}},
		{100, []int{5}},
	}
	for _, tc := range cases {
		probs := make([]HomeProblem, len(tc.raw))
		for i, g := range tc.raw {
			probs[i] = HomeProblem{ID: string(rune('a' + i)), Priority: i + 1, GainEstimate: g}
		}
		out := AssignCascadedGains(probs, tc.score)
		sum := 0
		for _, p := range out {
			sum += p.GainEstimate
			if p.GainEstimate < 0 {
				t.Fatalf("negative gain: %+v", p)
			}
		}
		room := 100 - tc.score
		if room < 0 {
			room = 0
		}
		if tc.score >= 95 {
			if sum != 0 {
				t.Fatalf("score %d: gains must be hidden, sum=%d", tc.score, sum)
			}
			continue
		}
		if sum > room {
			t.Fatalf("score %d raw %v: sum %d > room %d (out=%+v)", tc.score, tc.raw, sum, room, out)
		}
		if ScoreIfAllFixed(tc.score, out) > 100 {
			t.Fatalf("scoreIfAllFixed > 100")
		}
	}
}

func TestAssignCascadedGains_SecondCardUpTo(t *testing.T) {
	// 84 + 12 = 96, room left 4 → second raw 10 becomes 4 with GainUpTo
	probs := []HomeProblem{
		{ID: "disk_full", Priority: 10, GainEstimate: 12},
		{ID: "startup_heavy", Priority: 40, GainEstimate: 10},
	}
	out := AssignCascadedGains(probs, 84)
	if out[0].GainEstimate != 12 || out[0].GainUpTo {
		t.Fatalf("first: %+v", out[0])
	}
	if out[1].GainEstimate != 4 || !out[1].GainUpTo {
		t.Fatalf("second: %+v", out[1])
	}
	if ScoreIfAllFixed(84, out) != 100 {
		t.Fatalf("want 100, got %d", ScoreIfAllFixed(84, out))
	}
}
