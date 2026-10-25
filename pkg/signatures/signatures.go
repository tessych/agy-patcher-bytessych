package signatures

import (
	bin "github.com/tessych/agy-patcher-bytessych/pkg/binary"
	"github.com/tessych/agy-patcher-bytessych/pkg/patcher"
)

// Helper pattern constructors
func exact(b ...byte) []patcher.ByteMatcher {
	res := make([]patcher.ByteMatcher, len(b))
	for i, v := range b {
		res[i] = patcher.ExactByte(v)
	}
	return res
}

func anyByte(count int) []patcher.ByteMatcher {
	res := make([]patcher.ByteMatcher, count)
	for i := range res {
		res[i] = patcher.AnyByte{}
	}
	return res
}

func concat(parts ...[]patcher.ByteMatcher) patcher.Pattern {
	var total []patcher.ByteMatcher
	for _, p := range parts {
		total = append(total, p...)
	}
	return total
}

// CLI 1.4.0 x64 handleAuthResult:
// test rcx,rcx ; je ; cmp byte[rcx+8],0 ; jne eligible ;
// mov rax,rcx ; nop ; call IneligibilityFromResult ; spill rax/rbx/rcx at +0xe0/+0xe8/+0xf0.
// Repeating test rcx,rcx keeps ZF=0, so jne selects eligible without changing the formatter.
var (
	CLIGateX64 = &patcher.Gate{
		Original: concat(
			exact(0x48, 0x85, 0xc9, 0x0f, 0x84), anyByte(4),
			exact(0x80, 0x79, 0x08, 0x00, 0x0f, 0x85), anyByte(4),
			exact(0x48, 0x89, 0xc8, 0x90, 0xe8), anyByte(4),
			exact(0x48, 0x89, 0x84, 0x24, 0xe0, 0x00, 0x00, 0x00),
			exact(0x48, 0x89, 0x9c, 0x24, 0xe8, 0x00, 0x00, 0x00),
			exact(0x48, 0x89, 0x8c, 0x24, 0xf0, 0x00, 0x00, 0x00),
		),
		Patched: concat(
			exact(0x48, 0x85, 0xc9, 0x0f, 0x84), anyByte(4),
			exact(0x48, 0x85, 0xc9, 0x90, 0x0f, 0x85), anyByte(4),
			exact(0x48, 0x89, 0xc8, 0x90, 0xe8), anyByte(4),
			exact(0x48, 0x89, 0x84, 0x24, 0xe0, 0x00, 0x00, 0x00),
			exact(0x48, 0x89, 0x9c, 0x24, 0xe8, 0x00, 0x00, 0x00),
			exact(0x48, 0x89, 0x8c, 0x24, 0xf0, 0x00, 0x00, 0x00),
		),
		Fix:    []byte{0x48, 0x85, 0xc9, 0x90},
		Offset: 9,
		Desc:   "gate bypass (x64)",
		Arch:   bin.ArchX64,
	}

	// CLI 1.4.0 arm64:
	// ldrb w2,[x0,#8] ; tbnz w2,#0,eligible ;
	// bl IneligibilityFromResult ; stp x0-x7 at +0xe8/+0xf8/+0x108/+0x118.
	// mov w2,#1 selects the existing eligible branch.
	arm64BranchByte = patcher.ByteSet{0x02, 0x22, 0x42, 0x62, 0x82, 0xa2, 0xc2, 0xe2}
	arm64DisplByte  = patcher.ByteRange{Min: 0x00, Max: 0x07}
	arm64BlOpByte   = patcher.ByteRange{Min: 0x94, Max: 0x97}

	CLIGateARM64 = &patcher.Gate{
		Original: concat(
			exact(0x02, 0x20, 0x40, 0x39),
			[]patcher.ByteMatcher{arm64BranchByte, patcher.AnyByte{}, arm64DisplByte, patcher.ExactByte(0x37)},
			anyByte(3),
			[]patcher.ByteMatcher{arm64BlOpByte},
			exact(0xe0, 0x87, 0x0e, 0xa9, 0xe2, 0x8f, 0x0f, 0xa9, 0xe4, 0x97, 0x10, 0xa9, 0xe6, 0x9f, 0x11, 0xa9),
		),
		Patched: concat(
			exact(0x22, 0x00, 0x80, 0x52),
			[]patcher.ByteMatcher{arm64BranchByte, patcher.AnyByte{}, arm64DisplByte, patcher.ExactByte(0x37)},
			anyByte(3),
			[]patcher.ByteMatcher{arm64BlOpByte},
			exact(0xe0, 0x87, 0x0e, 0xa9, 0xe2, 0x8f, 0x0f, 0xa9, 0xe4, 0x97, 0x10, 0xa9, 0xe6, 0x9f, 0x11, 0xa9),
		),
		Fix:    []byte{0x22, 0x00, 0x80, 0x52},
		Offset: 0,
		Desc:   "gate bypass (arm64)",
		Arch:   bin.ArchARM64,
		Accept: patcher.CLIARM64Context,
	}

	CLIGate = &patcher.MultiGate{
		Gates: []*patcher.Gate{CLIGateX64, CLIGateARM64},
		Desc:  "CLI gate",
	}

	// Manager 2.19.1 Windows x64: force hasValidAuth and fall through to token attachment.
	// cmp byte[rax+8],0 ; je ; mov rdx,[rsp+0x70] ; mov [rax+0x60],rdx
	// Replacing cmp/je with mov byte[rax+8],1 ; nop*2 selects the success path.
	ManagerGateX64 = &patcher.Gate{
		Original: concat(
			exact(0x80, 0x78, 0x08, 0x00, 0x74), anyByte(1),
			exact(0x48, 0x8b, 0x54, 0x24, 0x70, 0x48, 0x89, 0x50, 0x60),
		),
		Patched: concat(
			exact(0xc6, 0x40, 0x08, 0x01, 0x90, 0x90),
			exact(0x48, 0x8b, 0x54, 0x24, 0x70, 0x48, 0x89, 0x50, 0x60),
		),
		Fix:    []byte{0xc6, 0x40, 0x08, 0x01, 0x90, 0x90},
		Offset: 0,
		Desc:   "hasValidAuth=true (x64)",
		Arch:   bin.ArchX64,
	}

	// Manager 2.19.1 Linux arm64:
	// ldrb w3,[x0,#8] ; tbz w3,#0,skip  ->  mov w3,#1 ; strb w3,[x0,#8]
	mgrArm64BranchByte = patcher.ByteSet{0x03, 0x23, 0x43, 0x63, 0x83, 0xa3, 0xc3, 0xe3}

	ManagerGateARM64 = &patcher.Gate{
		Original: concat(
			exact(0x03, 0x20, 0x40, 0x39),
			[]patcher.ByteMatcher{mgrArm64BranchByte, patcher.AnyByte{}, arm64DisplByte, patcher.ExactByte(0x36)},
			exact(0xe3, 0x13, 0x48, 0xa9, 0x03, 0x10, 0x06, 0xa9),
		),
		Patched: concat(
			exact(0x23, 0x00, 0x80, 0x52, 0x03, 0x20, 0x00, 0x39),
			exact(0xe3, 0x13, 0x48, 0xa9, 0x03, 0x10, 0x06, 0xa9),
		),
		Fix:    []byte{0x23, 0x00, 0x80, 0x52, 0x03, 0x20, 0x00, 0x39},
		Offset: 0,
		Desc:   "hasValidAuth=true (arm64)",
		Arch:   bin.ArchARM64,
	}

	ManagerGate = &patcher.MultiGate{
		Gates: []*patcher.Gate{ManagerGateX64, ManagerGateARM64},
		Desc:  "Manager language_server gate",
	}

	// Extension uses agy --hub (Manager x64 gate)
	ExtensionGate = &patcher.MultiGate{
		Gates: []*patcher.Gate{ManagerGateX64},
		Desc:  "Extension hub gate",
	}
)
