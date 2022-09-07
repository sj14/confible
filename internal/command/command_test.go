package command

import (
	"bytes"
	"os"
	"testing"

	"github.com/sj14/confible/internal/confible"
	"github.com/stretchr/testify/require"
)

func TestExecNoCache(t *testing.T) {
	tests := []struct {
		name       string
		cmd        string
		wantStdout string
		wantErr    bool
	}{
		{
			name:       "happy",
			cmd:        "echo 'Hello World'",
			wantStdout: "Hello World\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			if err := ExecNoCache(tt.cmd, stdout); (err != nil) != tt.wantErr {
				t.Errorf("ExecNoCache() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotStdout := stdout.String(); gotStdout != tt.wantStdout {
				t.Errorf("ExecNoCache() = %v, want %v", gotStdout, tt.wantStdout)
			}
		})
	}
}

func TestExec(t *testing.T) {
	type args struct {
		id        string
		commands  []confible.Command
		useCache  bool
		cachePath string
	}
	tests := []struct {
		name     string
		args     args
		wantErr  bool
		teardown func()
	}{
		{
			name: "happy no cache",
			args: args{
				id:       "happy no cache",
				commands: []confible.Command{{Exec: []string{"echo 'Hello World'"}}},
				useCache: false,
			},
		},
		{
			name: "happy with cache",
			args: args{
				id:        "happy with cache",
				commands:  []confible.Command{{Exec: []string{"echo 'Hello World'"}}},
				useCache:  true,
				cachePath: ".testcache",
			},
			teardown: func() { require.Nil(t, os.Remove(".testcache")) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if tt.teardown != nil {
					tt.teardown()
				}
			}()
			if err := Exec(tt.args.id, tt.args.commands, tt.args.useCache, tt.args.cachePath); (err != nil) != tt.wantErr {
				t.Errorf("Exec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
