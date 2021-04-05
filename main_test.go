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
			name: "untouched file",
			args: args{
				reader:     bytes.NewBufferString("first line\nsecond line"),
				comment:    "//",
				appendText: "new line 1\nnew line 2",
			},
			want: `first line
second line

// ~~~ CONFIBLE START ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
new line 1
new line 2
// ~~~ CONFIBLE END ~~~
`,
		},
		{
			name: "touched file",
			args: args{
				reader: bytes.NewBufferString(`
first line
second line

// ~~~ CONFIBLE START ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
existing line 1
existing line 2
existing line 3
// ~~~ CONFIBLE END ~~~
`),
				comment:    "//",
				appendText: "new line 1\nnew line 2",
			},
			want: `
first line
second line

// ~~~ CONFIBLE START ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
new line 1
new line 2
// ~~~ CONFIBLE END ~~~
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := appendContent(tt.args.reader, tt.args.comment, tt.args.appendText, tt.args.now)
			if (err != nil) != tt.wantErr {
				t.Errorf("appendContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			require.Equal(t, tt.want, got)
		})
	}
}
