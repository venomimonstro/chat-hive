package main

import (
    "errors"
    "testing"
    "time"
)

func TestSummarizeResultsUsesCompletedWallTime(t *testing.T) {
    started := time.Now().Add(-200 * time.Millisecond)
    samples := make(chan result, 2)
    samples <- result{Duration: 20 * time.Millisecond, Status: 200}
    samples <- result{Duration: 80 * time.Millisecond, Status: 201}
    close(samples)
    wall, latency, statuses, failures, bad := summarizeResults(samples, 2, started)
    if wall < 200*time.Millisecond { t.Fatalf("wall should include completed load duration, got %v", wall) }
    if bad || failures != 0 || len(latency) != 2 || latency[0] != 20*time.Millisecond || statuses[201] != 1 {
        t.Fatalf("unexpected report: latency=%v, statuses=%v, failures=%d, bad=%v", latency, statuses, failures, bad)
    }
}

func TestSummarizeResultsFailsClosedOnIncompleteOrHTTPError(t *testing.T) {
    samples := make(chan result, 2)
    samples <- result{Status: 302, Duration: time.Millisecond}
    samples <- result{Err: errors.New("network down")}
    close(samples)
    _, latency, _, failures, bad := summarizeResults(samples, 3, time.Now())
    if !bad || failures != 1 || len(latency) != 1 {
        t.Fatalf("unexpected incomplete/error state: bad=%v failures=%d completed=%d", bad, failures, len(latency))
    }
}
