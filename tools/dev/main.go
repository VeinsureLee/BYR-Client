package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"

	"github.com/VeinsureLee/BYR-Client/tools/internal/buildflow"
)

const usage = `BYR Client development workspace

  go run ./tools/dev check [--scope all|core|desktop|tools|gio-probe]
  go run ./tools/dev check android-core
  go run ./tools/dev run desktop
  go run ./tools/dev build desktop [--target windows/amd64] [--installer]
  go run ./tools/dev build android
  go run ./tools/dev build android-probe [--framework gio]
  go run ./tools/dev doctor

Desktop output: dist/desktop/<os>-<arch>/{app/,*.zip,manifest.json}
Android UI selection is pending: build android fails without creating an APK.
check android-core cross-compiles shared library packages; it is not an APK build.
android-probe writes an evaluation APK under dist/evaluation/android/gio/.
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if errors.Is(err, buildflow.ErrAndroidUndecided) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string) error {
	if len(arguments) == 0 || arguments[0] == "help" || arguments[0] == "--help" || arguments[0] == "-h" {
		fmt.Print(usage)
		return nil
	}
	root, err := buildflow.FindRoot(".")
	if err != nil {
		return err
	}
	engine := buildflow.Engine{Root: root, Run: buildflow.Exec(os.Stdout, os.Stderr), Output: os.Stdout}
	switch arguments[0] {
	case "check":
		if len(arguments) == 2 && arguments[1] == "android-core" {
			path, err := engine.CheckAndroidCore(ctx)
			if err == nil {
				fmt.Printf("Shared Go core compiles for android/arm64. No APK produced.\nReport: %s\n", path)
			}
			return err
		}
		flags := flag.NewFlagSet("check", flag.ContinueOnError)
		scope := flags.String("scope", "all", "all, core, desktop, tools or gio-probe")
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("unexpected check arguments")
		}
		return engine.Check(ctx, *scope)
	case "run":
		if len(arguments) != 2 || arguments[1] != "desktop" {
			return errors.New("usage: run desktop")
		}
		return engine.RunDesktop(ctx)
	case "build":
		if len(arguments) < 2 {
			return errors.New("usage: build desktop|android")
		}
		if arguments[1] == "android" {
			if len(arguments) != 2 {
				return errors.New("Android framework selection is pending; no build flags are available")
			}
			return engine.BuildAndroid()
		}
		if arguments[1] == "android-probe" {
			flags := flag.NewFlagSet("build android-probe", flag.ContinueOnError)
			framework := flags.String("framework", "gio", "evaluation framework (currently gio)")
			if err := flags.Parse(arguments[2:]); err != nil {
				return err
			}
			if flags.NArg() != 0 {
				return errors.New("unexpected probe build arguments")
			}
			path, err := engine.BuildAndroidProbe(ctx, *framework)
			if err == nil {
				fmt.Printf("Android evaluation APK packaged (not a production client): %s\n", path)
			}
			return err
		}
		if arguments[1] != "desktop" {
			return errors.New("unknown build target")
		}
		flags := flag.NewFlagSet("build desktop", flag.ContinueOnError)
		target := flags.String("target", runtime.GOOS+"/"+runtime.GOARCH, "desktop OS/architecture")
		installer := flags.Bool("installer", false, "also build a Windows installer through MyGo")
		if err := flags.Parse(arguments[2:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("unexpected desktop build arguments")
		}
		path, err := engine.BuildDesktop(ctx, *target, *installer)
		if err == nil {
			fmt.Printf("Desktop application packaged: %s\n", path)
		}
		return err
	case "doctor":
		if len(arguments) != 1 {
			return errors.New("usage: doctor")
		}
		return doctor(os.Stdout, engine)
	default:
		return fmt.Errorf("unknown command %q\n%s", arguments[0], usage)
	}
}

func doctor(output io.Writer, engine buildflow.Engine) error {
	fmt.Fprintf(output, "Workspace: %s\nGo toolchain: %s\nDesktop: MyGo (apps/desktop/go.mod pins its version)\n", engine.Root, runtime.Version())
	decision, err := engine.AndroidStatus()
	if err != nil {
		return err
	}
	fmt.Fprintf(output, "Android UI decision: %s\nCandidates: %v\n", decision.Status, decision.Candidates)
	fmt.Fprintf(output, "Evaluation priority: %v (not a framework selection)\n", decision.EvaluationPriority)
	sdk := os.Getenv("ANDROID_HOME")
	if sdk == "" {
		sdk = os.Getenv("ANDROID_SDK_ROOT")
	}
	if sdk == "" {
		fmt.Fprintln(output, "Android SDK: set ANDROID_HOME when evaluating Android UI")
	} else {
		fmt.Fprintf(output, "Android SDK: %s\n", sdk)
		ndks, _ := filepath.Glob(filepath.Join(sdk, "ndk", "*"))
		fmt.Fprintf(output, "Android NDK installations found: %d\n", len(ndks))
	}
	for _, tool := range []string{"java", "adb"} {
		path, err := exec.LookPath(tool)
		if err != nil {
			fmt.Fprintf(output, "%s: not on PATH\n", tool)
		} else {
			fmt.Fprintf(output, "%s: %s\n", tool, path)
		}
	}
	fmt.Fprintln(output, "SDK/NDK detection is informational; it does not certify either framework's Android toolchain.")
	return nil
}
