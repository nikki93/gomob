package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

var (
	target        = flag.String("target", "", "specify target (ios, tvos, android, js).\n")
	archNames     = flag.String("arch", "", "specify architecture(s) to include (arm, arm64, amd64).")
	minSDK        = flag.Int("minsdk", 16, "specify minimum supported Android platform sdk version (e.g. 28 for android28 a.k.a. Android 9 Pie).")
	buildMode     = flag.String("buildmode", "exe", "specify buildmode (archive, exe)")
	destPath      = flag.String("o", "", "output file or directory.\nFor -target ios or tvos, use the .app suffix to target simulators.")
	appID         = flag.String("appid", "", "app identifier (for -buildmode=exe)")
	iosHeader     = flag.String("iosheader", "ios.h", "input framework header (for -target=ios,tvos)")
	version       = flag.Int("version", 1, "app version (for -buildmode=exe)")
	printCommands = flag.Bool("x", false, "print the commands")
	keepWorkdir   = flag.Bool("work", false, "print the name of the temporary work directory and do not delete it when exiting.")
)

func main() {
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, "TODO: usage")
	}
	flag.Parse()
	if err := mainErr(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[0], err)
		os.Exit(1)
	}
	os.Exit(0)
}

func mainErr() error {
	pkg := flag.Arg(0)
	if pkg == "" {
		return errors.New("please a package")
	}
	pkgImportPath, err := runCmd(exec.Command("go", "list", "-f", "{{.ImportPath}}", pkg))
	if err != nil {
		return err
	}
	dir, err := runCmd(exec.Command("go", "list", "-f", "{{.Dir}}", pkg))
	if err != nil {
		return err
	}
	pkgImportPathElems := strings.Split(pkgImportPath, "/")

	bi := &BuildInfo{
		name:    pkgImportPathElems[len(pkgImportPathElems)-1],
		pkg:     pkg,
		target:  *target,
		appID:   *appID,
		dir:     dir,
		version: *version,
		minSDK:  *minSDK,
	}
	if bi.appID == "" {
		bi.appID = appIDFromPackage(pkgImportPath)
	}

	return Build(bi)
}

type BuildInfo struct {
	name    string
	pkg     string
	ldflags string
	target  string
	appID   string
	version int
	dir     string
	archs   []string
	minSDK  int
}

func Build(bi *BuildInfo) error {
	tmpDir, err := ioutil.TempDir("", "gogio-")
	if err != nil {
		return err
	}
	if *keepWorkdir {
		fmt.Fprintf(os.Stderr, "WORKDIR=%s\n", tmpDir)
	} else {
		defer os.RemoveAll(tmpDir)
	}

	if *archNames != "" {
		bi.archs = strings.Split(*archNames, ",")
	} else {
		switch *target {
		case "js":
			bi.archs = []string{"wasm"}
		case "ios", "tvos":
			// Only 64-bit support.
			bi.archs = []string{"arm64", "amd64"}
		case "android":
			bi.archs = []string{"arm", "arm64", "386", "amd64"}
		}
	}
	// TODO: Bring this back?
	//if appArgs := flag.Args()[1:]; len(appArgs) > 0 {
	//    // Pass along arguments to the app.
	//    bi.ldflags = fmt.Sprintf("-X gioui.org/app.extraArgs=%s", strings.Join(appArgs, "|"))
	//}

	switch *target {
	case "js":
		return errors.New("-target js unimplemented")
	case "ios", "tvos":
		return BuildIOSFramework(tmpDir, bi)
	case "android":
		return errors.New("-target android unimplemented")
	default:
		return errors.New("-target required")
	}
}

type arch struct {
	iosArch   string
	jniArch   string
	clangArch string
}

var allArchs = map[string]arch{
	"arm": {
		iosArch:   "armv7",
		jniArch:   "armeabi-v7a",
		clangArch: "armv7a-linux-androideabi",
	},
	"arm64": {
		iosArch:   "arm64",
		jniArch:   "arm64-v8a",
		clangArch: "aarch64-linux-android",
	},
	"386": {
		iosArch:   "i386",
		jniArch:   "x86",
		clangArch: "i686-linux-android",
	},
	"amd64": {
		iosArch:   "x86_64",
		jniArch:   "x86_64",
		clangArch: "x86_64-linux-android",
	},
}

func appIDFromPackage(pkgPath string) string {
	elems := strings.Split(pkgPath, "/")
	domain := strings.Split(elems[0], ".")
	name := ""
	if len(elems) > 1 {
		name = "." + elems[len(elems)-1]
	}
	if len(elems) < 2 && len(domain) < 2 {
		name = "." + domain[0]
		domain[0] = "localhost"
	} else {
		for i := 0; i < len(domain)/2; i++ {
			opp := len(domain) - 1 - i
			domain[i], domain[opp] = domain[opp], domain[i]
		}
	}

	pkgDomain := strings.Join(domain, ".")
	appid := []rune(pkgDomain + name)

	// a Java-language-style package name may contain upper- and lower-case
	// letters and underscores with individual parts separated by '.'.
	// https://developer.android.com/guide/topics/manifest/manifest-element
	for i, c := range appid {
		if !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' ||
			c == '_' || c == '.') {
			appid[i] = '_'
		}
	}
	return string(appid)
}

func runCmdRaw(cmd *exec.Cmd) ([]byte, error) {
	if *printCommands {
		fmt.Printf("%s\n", strings.Join(cmd.Args, " "))
	}
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	if err, ok := err.(*exec.ExitError); ok {
		return nil, fmt.Errorf("%s failed: %s%s", strings.Join(cmd.Args, " "), out, err.Stderr)
	}
	return nil, err
}

func runCmd(cmd *exec.Cmd) (string, error) {
	out, err := runCmdRaw(cmd)
	return string(bytes.TrimSpace(out)), err
}

func copyFile(dst, src string) (err error) {
	r, err := os.Open(src)
	if err != nil {
		return err
	}
	defer r.Close()
	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := w.Close(); err == nil {
			err = cerr
		}
	}()
	_, err = io.Copy(w, r)
	return err
}
