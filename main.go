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
	"path/filepath"
	"strings"

	"golang.org/x/sync/errgroup"
)

var (
	target        = flag.String("target", "", "specify target (ios, tvos, android, js).\n")
	archNames     = flag.String("arch", "", "specify architecture(s) to include (arm, arm64, amd64).")
	minSDK        = flag.Int("minsdk", 16, "specify minimum supported Android platform sdk version (e.g. 28 for android28 a.k.a. Android 9 Pie).")
	buildMode     = flag.String("buildmode", "exe", "specify buildmode (archive, exe)")
	destPath      = flag.String("o", "", "output file or directory.\nFor -target ios or tvos, use the .app suffix to target simulators.")
	appID         = flag.String("appid", "", "app identifier (for -buildmode=exe)")
	iosHeader     = flag.String("iosheader", "ios/framework.h", "input framework header (for -target=ios,tvos)")
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
		return errors.New("specify a package")
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

//
// Common
//

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
		panic("-target js unimplemented")
	case "ios", "tvos":
		return BuildIOSFramework(tmpDir, bi)
	case "android":
		panic("-target android unimplemented")
	default:
		panic("-target required")
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

//
// IOS
//

const minIOSVersion = "9.0"

func iosCompilerFor(target, arch string) (string, []string, error) {
	var platformSDK string
	var platformOS string
	switch target {
	case "ios":
		platformOS = "ios"
		platformSDK = "iphone"
	case "tvos":
		platformOS = "tvos"
		platformSDK = "appletv"
	}
	switch arch {
	case "arm", "arm64":
		platformSDK += "os"
	case "386", "amd64":
		platformOS += "-simulator"
		platformSDK += "simulator"
	default:
		return "", nil, fmt.Errorf("unsupported -arch: %s", arch)
	}
	sdkPath, err := runCmd(exec.Command("xcrun", "--sdk", platformSDK, "--show-sdk-path"))
	if err != nil {
		return "", nil, err
	}
	clang, err := runCmd(exec.Command("xcrun", "--sdk", platformSDK, "--find", "clang"))
	if err != nil {
		return "", nil, err
	}
	cflags := []string{
		"-fembed-bitcode",
		"-arch", allArchs[arch].iosArch,
		"-isysroot", sdkPath,
		"-m" + platformOS + "-version-min=" + minIOSVersion,
	}
	return clang, cflags, nil
}

func BuildIOSFramework(tmpDir string, bi *BuildInfo) error {
	frameworkRoot := *destPath
	if frameworkRoot == "" {
		frameworkRoot = fmt.Sprintf("%s.framework", strings.Title(bi.name))
	}
	target := bi.target
	framework := filepath.Base(frameworkRoot)
	const suf = ".framework"
	if !strings.HasSuffix(framework, suf) {
		return fmt.Errorf("the specified output %q does not end in '.framework'", frameworkRoot)
	}
	framework = framework[:len(framework)-len(suf)]
	if err := os.RemoveAll(frameworkRoot); err != nil {
		return err
	}
	frameworkDir := filepath.Join(frameworkRoot, "Versions", "A")
	for _, dir := range []string{"Headers", "Modules"} {
		p := filepath.Join(frameworkDir, dir)
		if err := os.MkdirAll(p, 0755); err != nil {
			return err
		}
	}
	symlinks := [][2]string{
		{"Versions/Current/Headers", "Headers"},
		{"Versions/Current/Modules", "Modules"},
		{"Versions/Current/" + framework, framework},
		{"A", filepath.Join("Versions", "Current")},
	}
	for _, l := range symlinks {
		if err := os.Symlink(l[0], filepath.Join(frameworkRoot, l[1])); err != nil && !os.IsExist(err) {
			return err
		}
	}
	exe := filepath.Join(frameworkDir, framework)
	lipo := exec.Command("xcrun", "lipo", "-o", exe, "-create")
	var builds errgroup.Group
	for _, a := range bi.archs {
		clang, cflags, err := iosCompilerFor(target, a)
		if err != nil {
			return err
		}
		lib := filepath.Join(tmpDir, "build-"+a)
		cmd := exec.Command(
			"go",
			"build",
			"-ldflags=-s -w "+bi.ldflags,
			"-buildmode=c-archive",
			"-o", lib,
			"-tags", "ios",
			bi.pkg,
		)
		lipo.Args = append(lipo.Args, lib)
		cflagsLine := strings.Join(cflags, " ")
		cmd.Env = append(
			os.Environ(),
			"GOOS=darwin",
			"GOARCH="+a,
			"CGO_ENABLED=1",
			"CC="+clang,
			"CGO_CFLAGS="+cflagsLine,
			"CGO_LDFLAGS="+cflagsLine,
		)
		builds.Go(func() error {
			_, err := runCmd(cmd)
			return err
		})
	}
	if err := builds.Wait(); err != nil {
		return err
	}
	if _, err := runCmd(lipo); err != nil {
		return err
	}
	headerDst := filepath.Join(frameworkDir, "Headers", framework+".h")
	if err := copyFile(headerDst, *iosHeader); err != nil {
		return err
	}
	module := fmt.Sprintf(`framework module "%s" {
    header "%[1]s.h"

    export *
}`, framework)
	moduleFile := filepath.Join(frameworkDir, "Modules", "module.modulemap")
	return ioutil.WriteFile(moduleFile, []byte(module), 0644)
}
