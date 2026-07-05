package redis

import (
	"bufio"
	"bytes"
	"reflect"
	"testing"
)

func TestReadReply(t *testing.T) {
	tests := []struct {
		name    string
		wire    string
		want    any
		wantErr bool
	}{
		{name: "simple string", wire: "+OK\r\n", want: "OK"},
		{name: "integer", wire: ":42\r\n", want: int64(42)},
		{name: "bulk string", wire: "$3\r\nabc\r\n", want: []byte("abc")},
		{name: "empty bulk", wire: "$0\r\n\r\n", want: []byte("")},
		{name: "null bulk", wire: "$-1\r\n", want: nil},
		{name: "null array", wire: "*-1\r\n", want: nil},
		{name: "error", wire: "-ERR nope\r\n", wantErr: true},
		{name: "unexpected prefix", wire: "?bad\r\n", wantErr: true},
		{
			name: "array of bulks",
			wire: "*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n",
			want: []any{[]byte("foo"), []byte("bar")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bufio.NewReader(bytes.NewReader([]byte(tt.wire)))
			got, err := readReply(r)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("readReply(%q) = %#v, want %#v", tt.wire, got, tt.want)
			}
		})
	}
}

func TestReadReplyTruncated(t *testing.T) {
	// A prefix with no following data must surface a read error, not a value.
	r := bufio.NewReader(bytes.NewReader(nil))
	if _, err := readReply(r); err == nil {
		t.Fatal("expected error on empty input")
	}
}

func TestReadReplyNestedArrayError(t *testing.T) {
	// An array that promises two elements but supplies only a truncated second
	// element must propagate the inner read error.
	r := bufio.NewReader(bytes.NewReader([]byte("*2\r\n:1\r\n$5\r\nab")))
	if _, err := readReply(r); err == nil {
		t.Fatal("expected error from truncated array element")
	}
}

func TestWriteCommandRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	if err := writeCommand(w, "SET", "key", "value"); err != nil {
		t.Fatalf("writeCommand: %v", err)
	}
	want := "*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n"
	if buf.String() != want {
		t.Fatalf("writeCommand encoded %q, want %q", buf.String(), want)
	}
}

func TestReadLineWithoutCR(t *testing.T) {
	// readLine must handle a bare "\n" terminator (no preceding "\r").
	r := bufio.NewReader(bytes.NewReader([]byte("hello\n")))
	line, err := readLine(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(line) != "hello" {
		t.Fatalf("readLine = %q, want %q", line, "hello")
	}
}
