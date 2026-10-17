package accountdata

import "testing"

func TestAccountExportConcurrencyLimit(t *testing.T) {
    handler := &HTTPHandler{exportSlots: make(chan struct{},2)}
    if !handler.acquireExportSlot() || !handler.acquireExportSlot() {
        t.Fatal("expected first two export slots")
    }
    if handler.acquireExportSlot() {
        t.Fatal("third export should fail fast rather than exhaust disk")
    }
    handler.releaseExportSlot()
    if !handler.acquireExportSlot() {
        t.Fatal("released slot must become available")
    }
}
