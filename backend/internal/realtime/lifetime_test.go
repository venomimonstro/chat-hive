package realtime
import (
 "testing"
 "time"
)
func TestRealtimeConnectionsCannotOutliveOriginalAuthenticationIndefinitely(t *testing.T) {
 if maxRealtimeConnectionAge < time.Minute || maxRealtimeConnectionAge > 10*time.Minute {
    t.Fatalf("realtime session must have a bounded, operationally safe lifetime: %v",maxRealtimeConnectionAge)
 }
}
