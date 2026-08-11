package appcatalog

import (
	"reflect"
	"testing"
)

func TestParseArguments(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []string
	}{
		{name: "cloudflare", line: "tunnel --no-autoupdate run --token abc", want: []string{"tunnel", "--no-autoupdate", "run", "--token", "abc"}},
		{name: "quoted", line: `serve --name "Home tunnel" 'literal value'`, want: []string{"serve", "--name", "Home tunnel", "literal value"}},
		{name: "empty quoted", line: `command ""`, want: []string{"command", ""}},
		{name: "escaped", line: `command one\ two`, want: []string{"command", "one two"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseArguments(test.line)
			if err != nil {
				t.Fatalf("ParseArguments: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("ParseArguments(%q) = %#v, want %#v", test.line, got, test.want)
			}
		})
	}
}

func TestParseArgumentsRejectsIncompleteSyntax(t *testing.T) {
	for _, line := range []string{`command "unfinished`, `command unfinished\`} {
		if _, err := ParseArguments(line); err == nil {
			t.Fatalf("ParseArguments(%q) unexpectedly succeeded", line)
		}
	}
}

func TestHasUnresolvedCredentialPlaceholder(t *testing.T) {
	for _, line := range []string{
		"tunnel run --token YOUR_TUNNEL_TOKEN_HERE",
		"serve --password CHANGE_ME",
		"serve --token <paste token here>",
	} {
		if !HasUnresolvedCredentialPlaceholder(line) {
			t.Fatalf("expected %q to contain an unresolved credential", line)
		}
	}
	if HasUnresolvedCredentialPlaceholder("tunnel run --token eyJhIjoiYWJjIn0.real-value") {
		t.Fatal("a concrete token was mistaken for a placeholder")
	}
}
