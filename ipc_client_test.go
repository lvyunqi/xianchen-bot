package main

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"testing"
)

func apiResult(id, value string) IPCMessage {
	return IPCMessage{
		Type:     "api_result",
		ID:       id,
		ValueB64: base64.StdEncoding.EncodeToString([]byte(value)),
	}
}

func TestIPCClientHandlesInterleavedEvent(t *testing.T) {
	responses := []IPCMessage{
		{Type: "event", ID: "6", Event: "private_message"},
		apiResult("api-1", "sent"),
	}
	written := make([]IPCMessage, 0, 2)
	client := newIPCClientForTest(
		func(message IPCMessage) error {
			written = append(written, message)
			return nil
		},
		func() (IPCMessage, error) {
			if len(responses) == 0 {
				return IPCMessage{}, fmt.Errorf("没有可读取的响应")
			}
			message := responses[0]
			responses = responses[1:]
			return message, nil
		},
	)
	client.handleInterleaved = func(message IPCMessage) error {
		return client.writeMessage(IPCMessage{Type: "event_result", ID: message.ID})
	}

	value, err := client.Call([]byte("send message"))
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if string(value) != "sent" {
		t.Fatalf("Call() = %q, want %q", value, "sent")
	}
	wantTypes := []string{"api_call", "event_result"}
	gotTypes := []string{written[0].Type, written[1].Type}
	if !reflect.DeepEqual(gotTypes, wantTypes) {
		t.Fatalf("写入顺序 = %v, want %v", gotTypes, wantTypes)
	}
}

func TestIPCClientDefersNestedEventUntilOuterAPIReturns(t *testing.T) {
	responses := []IPCMessage{
		{Type: "event", ID: "7", Event: "group_message"},
		apiResult("api-1", "outer sent"),
		apiResult("api-2", "deferred sent"),
	}
	written := make([]IPCMessage, 0, 3)
	deferred := make([]IPCMessage, 0, 1)
	client := newIPCClientForTest(
		func(message IPCMessage) error {
			written = append(written, message)
			return nil
		},
		func() (IPCMessage, error) {
			if len(responses) == 0 {
				return IPCMessage{}, fmt.Errorf("没有可读取的响应")
			}
			message := responses[0]
			responses = responses[1:]
			return message, nil
		},
	)
	client.handleInterleaved = func(message IPCMessage) error {
		deferred = append(deferred, message)
		return client.writeMessage(IPCMessage{Type: "event_result", ID: message.ID})
	}

	value, err := client.Call([]byte("outer message"))
	if err != nil {
		t.Fatalf("外层 Call() error = %v", err)
	}
	if string(value) != "outer sent" {
		t.Fatalf("外层 Call() = %q", value)
	}
	if len(deferred) != 1 || deferred[0].ID != "7" {
		t.Fatalf("排队事件 = %+v", deferred)
	}
	value, err = client.Call([]byte("deferred message"))
	if err != nil {
		t.Fatalf("延后 Call() error = %v", err)
	}
	if string(value) != "deferred sent" {
		t.Fatalf("延后 Call() = %q", value)
	}

	wantTypes := []string{"api_call", "event_result", "api_call"}
	gotTypes := make([]string, len(written))
	for i := range written {
		gotTypes[i] = written[i].Type
	}
	if !reflect.DeepEqual(gotTypes, wantTypes) {
		t.Fatalf("写入顺序 = %v, want %v", gotTypes, wantTypes)
	}
}
