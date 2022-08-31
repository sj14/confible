package variable

import (
	"reflect"
	"testing"
)

func TestOmitId(t *testing.T) {
	tests := []struct {
		name string
		vm variableMap
		want map[string]string
	}{
		{
			name:"",
			vm: variableMap{idVariable{id: "id", "variableName"}"asd"},
			
	}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := OmitId(tt.args.m); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("OmitId() = %v, want %v", got, tt.want)
			}
		})
	}
}
