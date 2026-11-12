package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/tessych/agy-patcher-bytessych/pkg/detector"
	"github.com/tessych/agy-patcher-bytessych/pkg/doctor"
	"github.com/tessych/agy-patcher-bytessych/pkg/patcher"
	"github.com/tessych/agy-patcher-bytessych/pkg/protect"
	"github.com/tessych/agy-patcher-bytessych/pkg/scripts"
	"github.com/tessych/agy-patcher-bytessych/pkg/signatures"
)

const version = "1.1.0"

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

func getExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func getExeName() string {
	exe, err := os.Executable()
	if err != nil {
		return "agy-patcher.exe"
	}
	base := filepath.Base(exe)
	if strings.HasSuffix(strings.ToLower(base), ".exe") {
		return base
	}
	return "agy-patcher.exe"
}

func getTargets(customCLI, customManager, customIDE, customExt string) []targetDef {
	found := detector.FindTargets()

	cliPaths := found.CLIs
	if customCLI != "" {
		// Разрешаем Scoop shim, если пользователь указал путь к шиму
		resolved := detector.ResolveScoopShim(customCLI)
		cliPaths = []string{resolved}
	} else if len(cliPaths) == 0 && found.CLI != "" {
		cliPaths = []string{found.CLI}
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

	var targets []targetDef

	if len(cliPaths) == 0 {
		targets = append(targets, targetDef{
			id:   patcher.TargetCLI,
			name: "Agy CLI",
			path: "",
			gate: signatures.CLIGate,
		})
	} else {
		for i, p := range cliPaths {
			name := "Agy CLI"
			if len(cliPaths) > 1 {
				name = fmt.Sprintf("Agy CLI (%d)", i+1)
			}
			targets = append(targets, targetDef{
				id:   patcher.TargetCLI,
				name: name,
				path: p,
				gate: signatures.CLIGate,
			})
		}
	}

	targets = append(targets,
		targetDef{id: patcher.TargetManager, name: "Agy", path: mgrPath, gate: signatures.ManagerGate},
		targetDef{id: patcher.TargetIDE, name: "Agy IDE", path: idePath, gate: nil},
		targetDef{id: patcher.TargetExtension, name: "Agy Extension", path: extPath, gate: signatures.ExtensionGate},
	)

	return targets
}

func main() {
	flagCLI := flag.String("path-cli", "", "Custom path to agy binary")
	flagManager := flag.String("path-manager", "", "Custom path to language_server binary")
	flagIDE := flag.String("path-ide", "", "Custom path to main.js")
	flagExt := flag.String("path-ext", "", "Custom path to extension agy binary")
	flagExtension := flag.String("path-extension", "", "Custom path to extension agy binary (alias for --path-ext)")
	flagNoProtect := flag.Bool("no-protect", false, "Disable automatic setup of AGY_CLI_DISABLE_AUTO_UPDATE=1 during patch")

	flag.Usage = func() {
		fmt.Printf("%sagy-patcher v%s by tessych%s\n\n", colorBold, version, colorReset)
		fmt.Println("Usage:")
		fmt.Println("  agy-patcher [status]              Check patch status of all components")
		fmt.Println("  agy-patcher patch [targets...]    Apply patch to targets (cli, manager, ide, extension)")
		fmt.Println("  agy-patcher restore [targets...]  Restore original files from *.agybak")
		fmt.Println("  agy-patcher doctor                Run system and Google API connectivity diagnostics")
		fmt.Println("  agy-patcher protect               Enable auto-updater protection (AGY_CLI_DISABLE_AUTO_UPDATE=1)")
		fmt.Println("  agy-patcher init-scripts [dir]    Generate Windows batch launcher scripts (patch.bat, etc.)")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *flagExt == "" && *flagExtension != "" {
		*flagExt = *flagExtension
	}

	// На Windows автоматически обеспечиваем наличие батников рядом с исполняемым файлом
	if runtime.GOOS == "windows" {
		_, _ = scripts.EnsureBatchScripts(getExeDir(), getExeName())
	}

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
		runPatch(targets, filter, *flagNoProtect)
	case "restore":
		filter := args[1:]
		runRestore(targets, filter)
	case "doctor":
		runDoctor(targets)
	case "protect":
		runProtect()
	case "init-scripts", "scripts", "init-bat":
		targetDir := ""
		if len(args) > 1 {
			targetDir = args[1]
		}
		runInitScripts(targetDir)
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
			flagHint := string(t.id)
			if t.id == patcher.TargetExtension {
				flagHint = "ext"
			}
			fmt.Printf("  %s[!!] not found (specify via --path-%s)%s\n\n", colorYellow, flagHint, colorReset)
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
			fmt.Printf("  status: %s[ok] patched%s\n\n", colorGreen, colorReset)
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

func runPatch(targets []targetDef, filter []string, noProtect bool) {
	selected := filterTargets(targets, filter)
	fmt.Printf("\n%sApplying patches...%s\n\n", colorBold, colorReset)

	var cliPatchedCount int

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
			if t.id == patcher.TargetCLI {
				cliPatchedCount++
			}
			continue
		}

		if err := p.ApplyPatch(); err != nil {
			fmt.Printf("  %s[FAIL] Patch failed: %v%s\n\n", colorRed, err, colorReset)
		} else {
			fmt.Printf("  %s[SUCCESS] Patched & verified! Backup created.%s\n\n", colorGreen, colorReset)
			if t.id == patcher.TargetCLI {
				cliPatchedCount++
			}
		}
	}

	// Автоматическая настройка защиты от автообновления для CLI
	if cliPatchedCount > 0 && !noProtect {
		if !protect.IsAutoUpdateDisabled() {
			fmt.Printf("%sConfiguring auto-updater protection (%s=1)...%s\n", colorCyan, protect.EnvDisableAutoUpdate, colorReset)
			res, err := protect.EnableAutoUpdateProtection()
			if err != nil {
				fmt.Printf("  %s[WARN] Failed to configure auto-updater protection automatically: %v%s\n", colorYellow, err, colorReset)
				fmt.Printf("        %sRun 'agy-patcher protect' or set %s=1 manually in your environment%s\n\n", colorYellow, protect.EnvDisableAutoUpdate, colorReset)
			} else {
				fmt.Printf("  %s[PROTECT] Auto-update protection enabled automatically! (%s)%s\n\n", colorGreen, res.Details, colorReset)
			}
		} else {
			fmt.Printf("%s[PROTECT] Auto-update protection is active (%s=1)%s\n\n", colorGreen, protect.EnvDisableAutoUpdate, colorReset)
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

func runProtect() {
	fmt.Printf("\n%sConfiguring Auto-Update Protection...%s\n\n", colorBold, colorReset)
	res, err := protect.EnableAutoUpdateProtection()
	if err != nil {
		fmt.Printf("%s[FAIL] Could not configure auto-updater protection: %v%s\n\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	if res.AlreadyActive {
		fmt.Printf("%s[OK] Auto-update protection is already configured.%s\n", colorGreen, colorReset)
		fmt.Printf("     %s\n\n", res.Details)
	} else {
		fmt.Printf("%s[SUCCESS] Configured auto-update protection successfully!%s\n", colorGreen, colorReset)
		fmt.Println("Target(s) updated:")
		for _, t := range res.Targets {
			fmt.Printf("  - %s\n", t)
		}
		fmt.Printf("\n%sOfficial agy CLI will no longer overwrite patched binaries with stock builds.%s\n\n", colorCyan, colorReset)
	}
}

func runInitScripts(targetDir string) {
	dir := targetDir
	if dir == "" {
		dir = getExeDir()
	}
	exeName := getExeName()

	fmt.Printf("\n%sGenerating Windows batch launcher scripts in %s...%s\n\n", colorBold, dir, colorReset)
	created, err := scripts.GenerateBatchScripts(dir, exeName)
	if err != nil {
		fmt.Printf("%s[FAIL] Could not generate scripts: %v%s\n\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	for _, p := range created {
		fmt.Printf("  %s[+] Created: %s%s\n", colorGreen, filepath.Base(p), colorReset)
	}
	fmt.Printf("\n%sReady! You can now double-click patch.bat, doctor.bat, or status.bat directly from File Explorer.%s\n\n", colorGreen, colorReset)
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

	fmt.Println("\n3. Environment & Auto-Update Protection:")
	envResults := doctor.RunEnvironmentDiagnostics()
	for _, env := range envResults {
		if env.IsOk {
			fmt.Printf("  - %-30s : %sENABLED%s (защита активна: %s)\n", env.Key, colorGreen, colorReset, env.Value)
		} else {
			fmt.Printf("  - %-30s : %sNOT SET%s (рекомендуется: %s)\n", env.Key, colorYellow, colorReset, env.Recommended)
			fmt.Printf("    %s↳ %s%s\n", colorYellow, env.Description, colorReset)
		}
	}

	fmt.Println()
	if allOk {
		fmt.Printf("%s[Doctor Verdict] Network connection to Google AI is healthy. Local patch will work seamlessly!%s\n\n", colorGreen, colorReset)
	} else {
		fmt.Printf("%s[Doctor Verdict] Some Google services are unreachable. If you are in a blocked region, please turn on your VPN or configure a proxy!%s\n\n", colorYellow, colorReset)
	}
}
