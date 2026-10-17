package realtime

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A malicious site must not be able to burn a browser client's short-lived,
// single-use ticket before websocket.Accept rejects its Origin.
func TestForeignOriginDoesNotConsumeTicket(t *testing.T) {
	tickets := NewTicketManager()
	token, _, err := tickets.Issue("user-1")
	if err != nil {
		t.Fatal(err)
	}
	handler := &HTTPHandler{tickets: tickets, originPattern: "chat.example.ru"}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/realtime?ticket="+token, nil)
	request.Header.Set("Origin", "https://malicious.example")
	recorder := httptest.NewRecorder()

	handler.connect(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
	if userID, valid := tickets.Consume(token); !valid || userID != "user-1" {
		t.Fatalf("valid ticket was consumed by a foreign origin; user=%q, valid=%v", userID, valid)
	}
}

func TestMalformedOriginDoesNotConsumeTicket(t *testing.T) {
	tickets := NewTicketManager()
	token, _, err := tickets.Issue("user-2")
	if err != nil {
		t.Fatal(err)
	}
	handler := &HTTPHandler{tickets: tickets, originPattern: "chat.example.ru"}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/realtime?ticket="+token, nil)
	request.Header.Set("Origin", "not a URL")
	recorder := httptest.NewRecorder()

	handler.connect(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
	if _, valid := tickets.Consume(token); !valid {
		t.Fatal("malformed origin consumed valid ticket")
	}
}

func TestInsecureSameHostOriginCannotBurnTicket(t *testing.T) {
	tickets := NewTicketManager()
	token, _, err := tickets.Issue("secure-user")
	if err != nil { t.Fatal(err) }
	handler := &HTTPHandler{tickets: tickets, originPattern: "chat.example.ru", originScheme: "https"}
	for _, origin := range []string{
		"http://chat.example.ru",
		"http://chat.example.ru:443",
		"https://chat.example.ru.evil.com",
		"https://attacker@chat.example.ru",
		"https://chat.example.ru/path",
		"https://chat.example.ru/?x=1",
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/realtime?ticket="+token, nil)
		req.Header.Set("Origin", origin)
		response := httptest.NewRecorder()
		handler.connect(response, req)
		if response.Code != http.StatusForbidden {
			t.Fatalf("origin %q unexpectedly accepted, status=%d", origin, response.Code)
		}
	}
	if userID, valid := tickets.Consume(token); !valid || userID != "secure-user" {
		t.Fatalf("invalid origins consumed valid ticket; user=%q valid=%v", userID, valid)
	}
}
