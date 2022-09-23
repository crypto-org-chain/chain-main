package types_test

import (
	"reflect"
	"testing"

	"github.com/crypto-org-chain/chain-main/v4/x/nft-transfer/types"
)

func TestIsAwayFromOrigin(t *testing.T) {
	type args struct {
		sourcePort    string
		sourceChannel string
		fullClassPath string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"transfer forward by origin chain", args{"p1", "c1", "kitty"}, true},
		{"transfer forward by relay chain", args{"p3", "c3", "p2/c2/kitty"}, true},
		{"transfer forward by relay chain", args{"p5", "c5", "p4/c4/p2/c2/kitty"}, true},
		{"transfer back by relay chain", args{"p6", "c6", "p6/c6/p4/c4/p2/c2/kitty"}, false},
		{"transfer back by relay chain", args{"p4", "c4", "p4/c4/p2/c2/kitty"}, false},
		{"transfer back by relay chain", args{"p2", "c2", "p2/c2/kitty"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := types.IsAwayFromOrigin(tt.args.sourcePort, tt.args.sourceChannel, tt.args.fullClassPath); got != tt.want {
				t.Errorf("IsAwayFromOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseClassTrace(t *testing.T) {
	type args struct {
		rawClassID string
	}
	tests := []struct {
		name string
		args args
		want types.ClassTrace
	}{
		{"native class", args{"kitty"}, types.ClassTrace{Path: "", BaseClassId: "kitty"}},
		{"transfer to (p2,c2)", args{"p2/c2/kitty"}, types.ClassTrace{Path: "p2/c2", BaseClassId: "kitty"}},
		{"transfer to (p4,c4)", args{"p4/c4/p2/c2/kitty"}, types.ClassTrace{Path: "p4/c4/p2/c2", BaseClassId: "kitty"}},
		{"transfer to (p6,c6)", args{"p6/c6/p4/c4/p2/c2/kitty"}, types.ClassTrace{Path: "p6/c6/p4/c4/p2/c2", BaseClassId: "kitty"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := types.ParseClassTrace(tt.args.rawClassID); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseClassTrace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassTrace_GetFullClassPath(t *testing.T) {
	tests := []struct {
		name string
		ct   types.ClassTrace
		want string
	}{
		{"native class", types.ClassTrace{Path: "", BaseClassId: "kitty"}, "kitty"},
		{"first  tranfer", types.ClassTrace{Path: "p2/c2", BaseClassId: "kitty"}, "p2/c2/kitty"},
		{"second tranfer", types.ClassTrace{Path: "p4/c4/p2/c2", BaseClassId: "kitty"}, "p4/c4/p2/c2/kitty"},
		{"third  tranfer", types.ClassTrace{Path: "p6/c6/p4/c4/p2/c2", BaseClassId: "kitty"}, "p6/c6/p4/c4/p2/c2/kitty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ct.GetFullClassPath(); got != tt.want {
				t.Errorf("ClassTrace.GetFullClassPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
