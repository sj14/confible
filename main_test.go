package main

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAppendContent(t *testing.T) {
	type args struct {
		reader     io.Reader
		id         string
		comment    string
		appendText string
		now        time.Time
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "empty file",
			args: args{
				reader:     bytes.NewBufferString(""),
				id:         "123",
				comment:    "//",
				appendText: "new line 1\nnew line 2",
			},
			want: `// ~~~ CONFIBLE START id: "123" ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
new line 1
new line 2
// ~~~ CONFIBLE END id: "123" ~~~
`,
		},
		{
			name: "untouched file",
			args: args{
				reader:     bytes.NewBufferString("first line\nsecond line"),
				id:         "123",
				comment:    "//",
				appendText: "new line 1\nnew line 2",
			},
			want: `first line
second line

// ~~~ CONFIBLE START id: "123" ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
new line 1
new line 2
// ~~~ CONFIBLE END id: "123" ~~~
`,
		},
		{
			name: "touched file",
			args: args{
				reader: bytes.NewBufferString(`
first line
second line

// ~~~ CONFIBLE START id: "123" ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
existing line 1
existing line 2
existing line 3
// ~~~ CONFIBLE END id: "123" ~~~
`),
				id:         "123",
				comment:    "//",
				appendText: "new line 1\nnew line 2",
			},
			want: `
first line
second line

// ~~~ CONFIBLE START id: "123" ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
new line 1
new line 2
// ~~~ CONFIBLE END id: "123" ~~~
`,
		},
		{
			name: "config with other id",
			args: args{
				reader: bytes.NewBufferString(`
first line
second line

// ~~~ CONFIBLE START id: "another config" ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
That's not your config yo!
Just leave me here!
// ~~~ CONFIBLE END id: "another config" ~~~
`),
				id:         "123",
				comment:    "//",
				appendText: "new line 1\nnew line 2",
			},
			want: `
first line
second line

// ~~~ CONFIBLE START id: "another config" ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
That's not your config yo!
Just leave me here!
// ~~~ CONFIBLE END id: "another config" ~~~

// ~~~ CONFIBLE START id: "123" ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
new line 1
new line 2
// ~~~ CONFIBLE END id: "123" ~~~
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := appendContent(tt.args.reader, tt.args.id, tt.args.comment, tt.args.appendText, tt.args.now)
			if (err != nil) != tt.wantErr {
				t.Errorf("appendContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			require.Equal(t, tt.want, got)
		})
	}
}

func TestAggregateConfigs(t *testing.T) {
	tests := []struct {
		name    string
		configs []config
		want    []config
	}{
		{
			name: "combine",
			configs: []config{
				{
					Comment: "#",
					Path:    "/tmp/test",
					Append:  "line 1\n",
				},
				{
					Comment: "//",
					Path:    "/tmp/test",
					Append:  "line 2\n",
				},
			},
			want: []config{
				{
					Comment: "#",
					Path:    "/tmp/test",
					Append:  "line 1\nline 2\n",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := aggregateConfigs(tt.configs)
			require.Equal(t, tt.want, got)
		})
	}
}
