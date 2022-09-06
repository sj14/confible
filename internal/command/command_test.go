package command

import (
	"bytes"
	"testing"

	"github.com/sj14/confible/internal/confible"
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
		id       string
		commands []confible.Command
		useCache bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "happy",
			args: args{
				id:       "happy",
				commands: []confible.Command{{Exec: []string{"echo 'Hello World'"}}},
				useCache: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Exec(tt.args.id, tt.args.commands, tt.args.useCache); (err != nil) != tt.wantErr {
				t.Errorf("Exec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
