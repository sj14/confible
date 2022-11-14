package config

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sj14/confible/internal/confible"
	"github.com/sj14/confible/internal/utils"
	"github.com/stretchr/testify/require"
)

func TestAppendContent(t *testing.T) {
	type args struct {
		reader     io.Reader
		id         string
		priority   int64
		comment    string
		appendText string
		now        time.Time
	}
	tests := []struct {
		name        string
		customSetup func()
		args        args
		want        string
		wantErr     bool
	}{
		{
			name: "templating",
			customSetup: func() {
				err := os.Setenv("TEST_ENV", "YOLO!!1")
				require.Nil(t, err)
			},
			args: args{
				reader:     bytes.NewBufferString(""),
				id:         "123",
				comment:    "//",
				appendText: "new line 1\n{{ .Env.TEST_ENV }}\nnew line 2",
			},
			want: `// ~~~ CONFIBLE START id: "123" priority: "1000" ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
new line 1
YOLO!!1
new line 2
// ~~~ CONFIBLE END id: "123" ~~~`,
		},
		{
			name: "empty file",
			args: args{
				reader:     bytes.NewBufferString(""),
				id:         "123",
				comment:    "//",
				appendText: "new line 1\nnew line 2",
			},
			want: `// ~~~ CONFIBLE START id: "123" priority: "1000" ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
new line 1
new line 2
// ~~~ CONFIBLE END id: "123" ~~~`,
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

// ~~~ CONFIBLE START id: "123" priority: "1000" ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
new line 1
new line 2
// ~~~ CONFIBLE END id: "123" ~~~`,
		},
		{
			name: "touched file",
			args: args{
				reader: bytes.NewBufferString(`
first line
second line

// ~~~ CONFIBLE START id: "123" priority: "1000" ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
existing line 1
existing line 2
existing line 3
// ~~~ CONFIBLE END id: "123" ~~~`),
				id:         "123",
				comment:    "//",
				appendText: "new line 1\nnew line 2",
			},
			want: `first line
second line

// ~~~ CONFIBLE START id: "123" priority: "1000" ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
new line 1
new line 2
// ~~~ CONFIBLE END id: "123" ~~~`,
		},
		{
			name: "config with other id higher",
			args: args{
				reader: bytes.NewBufferString(`
first line
second line

// ~~~ CONFIBLE START id: "another config" priority: "2" ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
That's not your config yo!
Just leave me here!
// ~~~ CONFIBLE END id: "another config" ~~~`),
				id:         "123",
				priority:   1,
				comment:    "//",
				appendText: "new line 1\nnew line 2",
			},
			want: `first line
second line

// ~~~ CONFIBLE START id: "123" priority: "1" ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
new line 1
new line 2
// ~~~ CONFIBLE END id: "123" ~~~

// ~~~ CONFIBLE START id: "another config" priority: "2" ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
That's not your config yo!
Just leave me here!
// ~~~ CONFIBLE END id: "another config" ~~~`,
		},
		{
			name: "config with other id lower",
			args: args{
				reader: bytes.NewBufferString(`
first line
second line

// ~~~ CONFIBLE START id: "another config" priority: "1" ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
That's not your config yo!
Just leave me here!
// ~~~ CONFIBLE END id: "another config" ~~~`),
				id:         "123",
				priority:   2,
				comment:    "//",
				appendText: "new line 1\nnew line 2",
			},
			want: `first line
second line

// ~~~ CONFIBLE START id: "another config" priority: "1" ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
That's not your config yo!
Just leave me here!
// ~~~ CONFIBLE END id: "another config" ~~~

// ~~~ CONFIBLE START id: "123" priority: "2" ~~~
// Mon, 01 Jan 0001 00:00:00 UTC
new line 1
new line 2
// ~~~ CONFIBLE END id: "123" ~~~`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.customSetup != nil {
				tt.customSetup()
			}

			got, err := modifyContent(tt.args.reader, tt.args.priority, tt.args.id, tt.args.comment, tt.args.appendText, TemplateData{Env: utils.GetEnvMap()}, tt.args.now)
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
		configs []confible.Config
		want    []confible.Config
	}{
		{
			name: "combine",
			configs: []confible.Config{
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
			want: []confible.Config{
				{
					Comment:  "#",
					Path:     "/tmp/test",
					Append:   "line 1\nline 2\n",
					Priority: DefaultPriority,
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

func TestFileContent(t *testing.T) {
	type args struct {
		reader io.Reader
		id     string
	}
	tests := []struct {
		name        string
		args        args
		wantContent string
		wantConfigs []confibleConfig
		wantErr     bool
	}{
		{
			name: "basic",
			args: args{
				reader: strings.NewReader(`
some stuff before

# ~~~ CONFIBLE START id: "zshrc" ~~~
# Sun, 04 Sep 2022 12:55:13 CEST
blablabka
# ~~~ CONFIBLE END id: "zshrc" ~~~	

some stuff after`),
				id: "another id",
			},
			wantContent: `some stuff before


some stuff after`,
			wantConfigs: []confibleConfig{
				{
					id:       "zshrc",
					priority: DefaultPriority,
					content:  "\n\n# ~~~ CONFIBLE START id: \"zshrc\" ~~~\n# Sun, 04 Sep 2022 12:55:13 CEST\nblablabka\n# ~~~ CONFIBLE END id: \"zshrc\" ~~~\t\n\n\n",
				},
			},
		},
		{
			name: "several configs",
			args: args{
				reader: strings.NewReader(`
some stuff before

# ~~~ CONFIBLE START id: "zshrc" priority: "55" ~~~
# Sun, 04 Sep 2022 12:55:13 CEST
blablabka
# ~~~ CONFIBLE END id: "zshrc" ~~~	

# ~~~ CONFIBLE START id: "other" priority: "44" ~~~
# Sun, 04 Sep 2022 12:55:14 CEST
yolo yolo
# ~~~ CONFIBLE END id: "other" ~~~	


some stuff after`),
				id: "another id",
			},
			wantContent: `some stuff before



some stuff after`,
			wantConfigs: []confibleConfig{
				{
					id:       "zshrc",
					priority: 55,
					content:  "\n\n# ~~~ CONFIBLE START id: \"zshrc\" priority: \"55\" ~~~\n# Sun, 04 Sep 2022 12:55:13 CEST\nblablabka\n# ~~~ CONFIBLE END id: \"zshrc\" ~~~\t\n\n\n",
				},
				{
					id:       "other",
					priority: 44,
					content:  "\n\n# ~~~ CONFIBLE START id: \"other\" priority: \"44\" ~~~\n# Sun, 04 Sep 2022 12:55:14 CEST\nyolo yolo\n# ~~~ CONFIBLE END id: \"other\" ~~~\t\n\n\n",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotContent, gotConfigs, err := fileContent(tt.args.reader)
			if (err != nil) != tt.wantErr {
				t.Errorf("fileContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.Equal(t, tt.wantContent, gotContent)
			require.Equal(t, tt.wantConfigs, gotConfigs)
		})
	}
}

func TestExtractPriority(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int64
	}{
		{
			name: "",
			s:    "# ~~~ CONFIBLE START id: \"zshrc\" priority: \"10\" ~~",
			want: 10,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractPriority(tt.s); got != tt.want {
				t.Errorf("extractPriority() = %v, want %v", got, tt.want)
			}
		})
	}
}
