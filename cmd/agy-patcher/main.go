package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tessych/agy-patcher-bytessych/pkg/detector"
	"github.com/tessych/agy-patcher-bytessych/pkg/doctor"
	"github.com/tessych/agy-patcher-bytessych/pkg/patcher"
	"github.com/tessych/agy-patcher-bytessych/pkg/signatures"
)

const version = "1.0.0"

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

type targetDef struct {
	id   patcher.AppTarget
	name string
	path string
	gate *patcher.MultiGate
}

func getTargets(customCLI, customManager, customIDE, customExt string) []targetDef {
	found := detector.FindTargets()

	cliPath := found.CLI
	if customCLI != "" {
		cliPath = customCLI
	}

	mgrPath := found.Manager
	if customManager != "" {
		mgrPath = customManager
	}

	idePath := found.IDE
	if customIDE != "" {
		idePath = customIDE
	}

	extPath := found.Extension
	if customExt != "" {
		extPath = customExt
	}

	return []targetDef{
		{id: patcher.TargetCLI, name: "Agy CLI", path: cliPath, gate: signatures.CLIGate},
		{id: patcher.TargetManager, name: "Agy", path: mgrPath, gate: signatures.ManagerGate},
		{id: patcher.TargetIDE, name: "Agy IDE", path: idePath, gate: nil},
		{id: patcher.TargetExtension, name: "Agy Extension", path: extPath, gate: signatures.ExtensionGate},
	}
}

func main() {
	flagCLI := flag.String("path-cli", "", "Custom path to agy binary")
	flagManager := flag.String("path-manager", "", "Custom path to language_server binary")
	flagIDE := flag.String("path-ide", "", "Custom path to main.js")
	flagExt := flag.String("path-ext", "", "Custom path to extension agy binary")

	flag.Usage = func() {
		fmt.Printf("%sagy-patcher v%s by tessych%s\n\n", colorBold, version, colorReset)
		fmt.Println("Usage:")
		fmt.Println("  agy-patcher [status]              Check patch status of all components")
		fmt.Println("  agy-patcher patch [targets...]    Apply patch to targets (cli, manager, ide, extension)")
		fmt.Println("  agy-patcher restore [targets...]  Restore original files from *.agybak")
		fmt.Println("  agy-patcher doctor                Run system and Google API connectivity diagnostics")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
	}

	flag.Parse()

	args := flag.Args()
	cmd := "status"
	if len(args) > 0 {
		cmd = strings.ToLower(args[0])
	}

	targets := getTargets(*flagCLI, *flagManager, *flagIDE, *flagExt)

	switch cmd {
	case "status":
		runStatus(targets)
	case "patch":
		filter := args[1:]
		runPatch(targets, filter)
	case "restore":
		filter := args[1:]
		runRestore(targets, filter)
	case "doctor":
		runDoctor(targets)
	case "help", "-h", "--help":
		flag.Usage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s. Use 'agy-patcher help'\n", cmd)
		os.Exit(1)
	}
}

func runStatus(targets []targetDef) {
	fmt.Printf("\n%sagy-patcher status%s\n\n", colorBold, colorReset)

	for _, t := range targets {
		fmt.Printf("=== %s%s%s ===\n", colorCyan, t.name, colorReset)
		if t.path == "" {
			fmt.Printf("  %s[!!] not found (specify via --path-%s)%s\n\n", colorYellow, t.id, colorReset)
			continue
		}

		fmt.Printf("  target: %s\n", t.path)
		p := &patcher.Patcher{Target: t.id, Path: t.path, Gate: t.gate}
		status, _, err := p.GetStatus()
		if err != nil {
			fmt.Printf("  status: %serror (%v)%s\n\n", colorRed, err, colorReset)
			continue
		}

		switch status {
		case patcher.StatusPatched:
			fmt.Printf("  status: %s[ok] patched (eligibility bypass active)%s\n\n", colorGreen, colorReset)
		case patcher.StatusUnpatched:
			fmt.Printf("  status: %s[--] unpatched (ready to patch)%s\n\n", colorYellow, colorReset)
		default:
			fmt.Printf("  status: %s[??] unknown signature%s\n\n", colorRed, colorReset)
		}
	}
}

func filterTargets(targets []targetDef, filter []string) []targetDef {
	if len(filter) == 0 {
		return targets
	}
	set := make(map[string]bool)
	for _, f := range filter {
		set[strings.ToLower(f)] = true
	}
	var out []targetDef
	for _, t := range targets {
		if set[string(t.id)] || set[strings.ToLower(t.name)] {
			out = append(out, t)
		}
	}
	return out
}

func runPatch(targets []targetDef, filter []string) {
	selected := filterTargets(targets, filter)
	fmt.Printf("\n%sApplying patches...%s\n\n", colorBold, colorReset)

	for _, t := range selected {
		if t.path == "" {
			fmt.Printf("Skipping %s: not found\n", t.name)
			continue
		}

		fmt.Printf("Patching %s (%s)...\n", t.name, t.path)
		p := &patcher.Patcher{Target: t.id, Path: t.path, Gate: t.gate}

		status, _, err := p.GetStatus()
		if err != nil {
			fmt.Printf("  %s[FAIL] Could not read target: %v%s\n\n", colorRed, err, colorReset)
			continue
		}

		if status == patcher.StatusPatched {
			fmt.Printf("  %s[SKIP] Already patched%s\n\n", colorGreen, colorReset)
			continue
		}

		if err := p.ApplyPatch(); err != nil {
			fmt.Printf("  %s[FAIL] Patch failed: %v%s\n\n", colorRed, err, colorReset)
		} else {
			fmt.Printf("  %s[SUCCESS] Patched & verified! Backup created.%s\n\n", colorGreen, colorReset)
		}
	}
}

func runRestore(targets []targetDef, filter []string) {
	selected := filterTargets(targets, filter)
	fmt.Printf("\n%sRestoring originals from *.agybak...%s\n\n", colorBold, colorReset)

	for _, t := range selected {
		if t.path == "" {
			continue
		}

		fmt.Printf("Restoring %s...\n", t.name)
		p := &patcher.Patcher{Target: t.id, Path: t.path, Gate: t.gate}

		if err := p.Restore(); err != nil {
			fmt.Printf("  %s[FAIL] Restore failed: %v%s\n\n", colorRed, err, colorReset)
		} else {
			fmt.Printf("  %s[SUCCESS] Restored original!%s\n\n", colorGreen, colorReset)
		}
	}
}

func runDoctor(targets []targetDef) {
	fmt.Printf("\n%sRunning agy-patcher diagnostics...%s\n\n", colorBold, colorReset)

	fmt.Println("1. Component Local Status:")
	for _, t := range targets {
		if t.path == "" {
			fmt.Printf("  - %-32s : %smissing%s\n", t.name, colorYellow, colorReset)
			continue
		}
		p := &patcher.Patcher{Target: t.id, Path: t.path, Gate: t.gate}
		st, _, _ := p.GetStatus()
		color := colorYellow
		if st == patcher.StatusPatched {
			color = colorGreen
		} else if st == patcher.StatusUnknown {
			color = colorRed
		}
		fmt.Printf("  - %-32s : %s%s%s (%s)\n", t.name, color, st, colorReset, t.path)
	}

	fmt.Println("\n2. Google Cloud / AI Network Reachability:")
	netResults := doctor.RunNetworkDiagnostics()
	allOk := true
	for _, res := range netResults {
		if res.Success {
			fmt.Printf("  - %-20s : %sOK%s (%v)\n", res.Name, colorGreen, colorReset, res.Latency.Round(time.Millisecond))
		} else {
			allOk = false
			fmt.Printf("  - %-20s : %sBLOCKED / ERROR%s (%s)\n", res.Name, colorRed, colorReset, res.Error)
		}
	}

	fmt.Println()
	if allOk {
		fmt.Printf("%s[Doctor Verdict] Network connection to Google AI is healthy. Local patch will work seamlessly!%s\n\n", colorGreen, colorReset)
	} else {
		fmt.Printf("%s[Doctor Verdict] Some Google services are unreachable. If you are in a blocked region, please turn on your VPN or configure a proxy!%s\n\n", colorYellow, colorReset)
	}
}
