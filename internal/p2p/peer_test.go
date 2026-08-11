package p2p

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDirectConnectionSurvivesAfterSignaling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	offerPeer, offer, err := NewOffer(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	defer offerPeer.Close()
	answerPeer, answer, err := Answer(ctx, "", offer)
	if err != nil {
		t.Fatal(err)
	}
	defer answerPeer.Close()
	if err := offerPeer.SetAnswer(answer); err != nil {
		t.Fatal(err)
	}

	type connectionResult struct {
		conn *Conn
		err  error
	}
	offerResult := make(chan connectionResult, 1)
	go func() {
		conn, connectErr := offerPeer.Connect(ctx)
		offerResult <- connectionResult{conn: conn, err: connectErr}
	}()
	answerConn, err := answerPeer.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	offerConnected := <-offerResult
	if offerConnected.err != nil {
		t.Fatal(offerConnected.err)
	}
	offerConn := offerConnected.conn
	defer offerConn.Close()
	defer answerConn.Close()

	// The offer/answer strings are deliberately gone at this point. The
	// established channel must carry a multi-frame message without any
	// coordinator or relay remaining in the data path.
	payload := strings.Repeat("direct-faroos-", 12_000)
	writeErr := make(chan error, 1)
	go func() {
		writeErr <- offerConn.WriteJSON(map[string]string{"payload": payload})
	}()
	if err := answerConn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var received map[string]string
	if err := answerConn.ReadJSON(&received); err != nil {
		t.Fatal(err)
	}
	if err := <-writeErr; err != nil {
		t.Fatal(err)
	}
	if received["payload"] != payload {
		t.Fatalf("direct payload mismatch: got %d bytes, want %d", len(received["payload"]), len(payload))
	}

	if err := answerConn.SetReadDeadline(time.Now().Add(250 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if err := answerConn.ReadJSON(&received); err == nil {
		t.Fatal("P2P read did not honor its deadline")
	}
}
