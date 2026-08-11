package stunserver

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/pion/stun/v3"
)

func TestBindingRequest(t *testing.T) {
	server, err := New("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	port := server.Addr().(*net.UDPAddr).Port
	uri, err := stun.ParseURI(fmt.Sprintf("stun:127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	client, err := stun.DialURI(uri, &stun.DialConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	result := make(chan error, 1)
	request := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	if err := client.Do(request, func(event stun.Event) {
		if event.Error != nil {
			result <- event.Error
			return
		}
		var address stun.XORMappedAddress
		result <- address.GetFrom(event.Message)
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for STUN binding response")
	}
}
