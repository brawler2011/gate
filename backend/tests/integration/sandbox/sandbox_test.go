//go:build integration

package sandbox_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/syslib"
	"github.com/brawler2011/gate/backend/tests/judgeimage"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestIntegrationSandbox(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("Starting go-judge container...")
	image, err := judgeimage.Ref()
	if err != nil {
		t.Fatalf("Failed to name go-judge image: %v", err)
	}
	req := testcontainers.ContainerRequest{
		Image:        image,
		ExposedPorts: []string{"5051/tcp"},
		// The internal check execs into the container, and that process can block go-judge's cgroup setup.
		WaitingFor: wait.ForListeningPort("5051/tcp").SkipInternalCheck(),
		Privileged: true,
		Cmd:        []string{"-enable-grpc", "-grpc-addr=0.0.0.0:5051", "-no-fallback"},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start go-judge container: %v", err)
	}
	defer func() {
		if err := container.Terminate(context.Background(), testcontainers.StopTimeout(0)); err != nil {
			t.Errorf("Failed to terminate container: %v", err)
		}
	}()

	ip, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get container host: %v", err)
	}

	port, err := container.MappedPort(ctx, "5051")
	if err != nil {
		t.Fatalf("Failed to get container mapped port: %v", err)
	}

	grpcAddr := fmt.Sprintf("%s:%s", ip, port.Port())
	t.Logf("go-judge container started at gRPC address: %s", grpcAddr)

	client, err := sandbox.NewClient(grpcAddr)
	if err != nil {
		t.Fatalf("Failed to create sandbox client: %v", err)
	}
	defer client.Close()

	cfg, err := sandbox.LoadConfig("../../../../judge/langs.toml")
	if err != nil {
		t.Fatalf("Failed to load languages config: %v", err)
	}

	sb := sandbox.NewSandbox(client, cfg)

	for i := 0; i < 30; i++ {
		probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		_, err := sb.Compile(probeCtx, []byte("int main(){return 0;}"), "cpp17", nil)
		cancel()
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	gfmtRoot := "../../../pkg/testdata/gfmt"
	entries, err := os.ReadDir(gfmtRoot)
	if err != nil {
		t.Fatalf("failed to read gfmt directory: %v", err)
	}

	t.Run("packages", func(t *testing.T) {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			pkgName := entry.Name()
			pkgPath := filepath.Join(gfmtRoot, pkgName)

			t.Run(pkgName, func(t *testing.T) {
				t.Parallel()
				pkg, err := gfmt.OpenPackage(pkgPath)
				if err != nil {
					t.Fatalf("failed to open package %s: %v", pkgName, err)
				}

				deps := make(map[string][]byte)
				libDir := filepath.Join(pkgPath, "lib")
				if libFiles, err := os.ReadDir(libDir); err == nil {
					for _, f := range libFiles {
						if !f.IsDir() {
							content, err := os.ReadFile(filepath.Join(libDir, f.Name()))
							if err != nil {
								t.Fatalf("failed to read lib file %s: %v", f.Name(), err)
							}
							deps[f.Name()] = content
						}
					}
				}

				for _, name := range pkg.Problem.SystemLibrary {
					system, ok := syslib.Lookup(name)
					if !ok {
						t.Fatalf("unknown system library file %s", name)
					}
					deps[name] = []byte(system.Content)
				}

				var checker sandbox.Executable
				checkersDir := filepath.Join(pkgPath, "checkers")
				if checkerFiles, err := os.ReadDir(checkersDir); err == nil && len(checkerFiles) > 0 {
					for _, f := range checkerFiles {
						if !f.IsDir() {
							langKey, err := getLanguageKey(f.Name())
							if err != nil {
								continue
							}
							code, err := os.ReadFile(filepath.Join(checkersDir, f.Name()))
							if err != nil {
								t.Fatalf("failed to read checker file %s: %v", f.Name(), err)
							}
							exe, err := sb.Compile(ctx, code, langKey, deps)
							if err != nil {
								t.Fatalf("failed to compile checker %s: %v", f.Name(), err)
							}
							checker = exe
							break
						}
					}
				}

				var interactor sandbox.Executable
				interactorsDir := filepath.Join(pkgPath, "interactors")
				if interactorFiles, err := os.ReadDir(interactorsDir); err == nil && len(interactorFiles) > 0 {
					for _, f := range interactorFiles {
						if !f.IsDir() {
							langKey, err := getLanguageKey(f.Name())
							if err != nil {
								continue
							}
							code, err := os.ReadFile(filepath.Join(interactorsDir, f.Name()))
							if err != nil {
								t.Fatalf("failed to read interactor file %s: %v", f.Name(), err)
							}
							exe, err := sb.Compile(ctx, code, langKey, deps)
							if err != nil {
								t.Fatalf("failed to compile interactor %s: %v", f.Name(), err)
							}
							interactor = exe
							break
						}
					}
				}

				generators := make(map[string]sandbox.Executable)
				generatorsDir := filepath.Join(pkgPath, "generators")
				if genFiles, err := os.ReadDir(generatorsDir); err == nil {
					for _, f := range genFiles {
						if !f.IsDir() {
							langKey, err := getLanguageKey(f.Name())
							if err != nil {
								continue
							}
							code, err := os.ReadFile(filepath.Join(generatorsDir, f.Name()))
							if err != nil {
								t.Fatalf("failed to read generator file %s: %v", f.Name(), err)
							}
							exe, err := sb.Compile(ctx, code, langKey, deps)
							if err != nil {
								t.Fatalf("failed to compile generator %s: %v", f.Name(), err)
							}
							nameWithoutExt := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))
							generators[nameWithoutExt] = exe
						}
					}
				}

				type TestCase struct {
					Name     string
					Manual   string
					Generate string
				}
				var testCases []TestCase
				var subtaskKeys []string
				for k := range pkg.Problem.Subtasks {
					subtaskKeys = append(subtaskKeys, k)
				}
				sort.Slice(subtaskKeys, func(i, j int) bool {
					if subtaskKeys[i] == "samples" {
						return true
					}
					if subtaskKeys[j] == "samples" {
						return false
					}
					return subtaskKeys[i] < subtaskKeys[j]
				})

				for _, subtaskKey := range subtaskKeys {
					subtask := pkg.Problem.Subtasks[subtaskKey]
					for idx, tc := range subtask.Tests {
						name := fmt.Sprintf("%s_%d", subtaskKey, idx+1)
						testCases = append(testCases, TestCase{
							Name:     name,
							Manual:   tc.Manual,
							Generate: tc.Generate,
						})
					}
				}

				getInput := func(tc TestCase) ([]byte, error) {
					if tc.Manual != "" {
						return pkg.GetTestInput(tc.Manual)
					}
					parts := strings.Fields(tc.Generate)
					if len(parts) == 0 {
						return nil, fmt.Errorf("empty generate command")
					}
					genName := parts[0]
					args := parts[1:]
					genID, ok := generators[genName]
					if !ok {
						return nil, fmt.Errorf("generator not found: %s", genName)
					}
					return sb.Generate(ctx, genID, args)
				}

				solutionsDir := filepath.Join(pkgPath, "solutions")
				solFiles, err := os.ReadDir(solutionsDir)
				if err != nil {
					t.Fatalf("failed to read solutions directory: %v", err)
				}

				type Solution struct {
					Filename string
					Lang     string
					Expected string
				}
				var solutions []Solution
				var referenceSol *Solution

				for _, f := range solFiles {
					if f.IsDir() {
						continue
					}
					expected, ok := expectedVerdict(f.Name())
					if !ok {
						continue
					}
					langKey, err := getLanguageKey(f.Name())
					if err != nil {
						t.Logf("Skipping solution %s: %v", f.Name(), err)
						continue
					}
					sol := Solution{
						Filename: f.Name(),
						Lang:     langKey,
						Expected: expected,
					}
					solutions = append(solutions, sol)
					if expected == "OK" && referenceSol == nil {
						referenceSol = &sol
					}
				}

				if referenceSol == nil {
					t.Fatalf("No reference OK solution found in %s", pkgPath)
				}

				refCode, err := pkg.GetSolution(referenceSol.Filename)
				if err != nil {
					t.Fatalf("failed to read reference solution %s: %v", referenceSol.Filename, err)
				}
				refSolID, err := sb.Compile(ctx, refCode, referenceSol.Lang, nil)
				if err != nil {
					t.Fatalf("failed to compile reference solution %s: %v", referenceSol.Filename, err)
				}

				type TestCaseData struct {
					TC     TestCase
					Input  []byte
					Answer []byte
				}
				var testCasesData []TestCaseData
				for _, tc := range testCases {
					input, err := getInput(tc)
					if err != nil {
						t.Fatalf("failed to get input for test case %s: %v", tc.Name, err)
					}
					var answer []byte
					if pkg.Problem.Type != "interactive" {
						timeLimit := pkg.Problem.Limits.TimeMs
						if timeLimit <= 0 {
							timeLimit = 1000
						}
						memLimit := pkg.Problem.Limits.MemoryMb
						if memLimit <= 0 {
							memLimit = 256
						}
						solResult, err := sb.Test(ctx, refSolID, referenceSol.Lang, input, timeLimit, memLimit)
						if err != nil {
							t.Fatalf("failed to run reference solution on test %s: %v", tc.Name, err)
						}
						if solResult.Status != sandbox.StatusOK {
							t.Fatalf("reference solution did not return OK on test %s: status=%s, stderr=%s",
								tc.Name, solResult.Status, string(solResult.Stderr))
						}
						answer = solResult.Stdout
					}
					testCasesData = append(testCasesData, TestCaseData{
						TC:     tc,
						Input:  input,
						Answer: answer,
					})
				}

				for _, sol := range solutions {
					t.Run(sol.Filename, func(t *testing.T) {
						t.Parallel()
						solCode, err := pkg.GetSolution(sol.Filename)
						if err != nil {
							t.Fatalf("failed to read solution %s: %v", sol.Filename, err)
						}

						solID, err := sb.Compile(ctx, solCode, sol.Lang, nil)
						if sol.Expected == "CE" {
							if err == nil {
								t.Errorf("expected compilation error for %s, but compiled successfully", sol.Filename)
							}
							return
						}
						if err != nil {
							t.Fatalf("failed to compile solution %s: %v", sol.Filename, err)
						}

						finalStatus := sandbox.StatusOK
						var lastStderr []byte
						var lastExitStatus int
						for _, tcd := range testCasesData {
							timeLimit := pkg.Problem.Limits.TimeMs
							if timeLimit <= 0 {
								timeLimit = 1000
							}
							memLimit := pkg.Problem.Limits.MemoryMb
							if memLimit <= 0 {
								memLimit = 256
							}

							if pkg.Problem.Type == "interactive" {
								interactRes, err := sb.Interact(ctx, solID, sol.Lang, interactor, tcd.Input, timeLimit, memLimit)
								if err != nil {
									t.Fatalf("interact failed: %v", err)
								}
								if interactRes.Status != sandbox.StatusOK {
									finalStatus = interactRes.Status
									lastStderr = interactRes.SolutionResult.Stderr
									lastExitStatus = interactRes.SolutionResult.ExitStatus
									break
								}
							} else {
								runRes, err := sb.Test(ctx, solID, sol.Lang, tcd.Input, timeLimit, memLimit)
								if err != nil {
									t.Fatalf("solution execution failed: %v", err)
								}
								if runRes.Status != sandbox.StatusOK {
									finalStatus = runRes.Status
									lastStderr = runRes.Stderr
									lastExitStatus = runRes.ExitStatus
									break
								}
								checkRes, err := sb.Check(ctx, checker, tcd.Input, runRes.Stdout, tcd.Answer)
								if err != nil {
									t.Fatalf("checker failed: %v", err)
								}
								if checkRes.Status != sandbox.StatusOK {
									finalStatus = checkRes.Status
									break
								}
							}
						}

						if string(finalStatus) != sol.Expected {
							if sol.Expected == "MLE" && finalStatus == sandbox.StatusRE {
								t.Logf("Solution %s evaluated as RE (accepted fallback for MLE in rlimit environments)", sol.Filename)
							} else {
								t.Errorf("verdict mismatch for %s: expected %s, got %s (ExitStatus: %d, Stderr: %q)",
									sol.Filename, sol.Expected, finalStatus, lastExitStatus, string(lastStderr))
							}
						} else {
							t.Logf("Solution %s evaluated as expected: %s", sol.Filename, finalStatus)
						}
					})
				}
			})
		}
	})

	// The image ships a jlink runtime; submissions still reach the modules of java.se.
	t.Run("java standard modules", func(t *testing.T) {
		source := `import java.awt.Point;
import java.sql.Timestamp;
import java.util.Locale;
import java.util.Scanner;
import java.util.logging.Logger;

public class Main {
    public static void main(String[] args) {
        Logger.getLogger("main").fine("unused");
        // Locale data beyond US English lives in jdk.localedata.
        double half = new Scanner("0,5").useLocale(Locale.GERMANY).nextDouble();
        System.out.println(new Point(1, 2).y + new Timestamp(0).getNanos() + half);
    }
}`
		executable, err := sb.Compile(ctx, []byte(source), "java", nil)
		if err != nil {
			t.Fatalf("failed to compile: %v", err)
		}
		result, err := sb.Test(ctx, executable, "java", nil, 5000, 256)
		if err != nil {
			t.Fatalf("failed to run: %v", err)
		}
		if result.Status != sandbox.StatusOK || strings.TrimSpace(string(result.Stdout)) != "2.5" {
			t.Fatalf("got %s with output %q (stderr %q)", result.Status, result.Stdout, result.Stderr)
		}
	})

	// Every catalog configuration compiles and runs a program in the image.
	t.Run("catalog configurations", func(t *testing.T) {
		sources := map[string]string{
			"c":       "#include <stdio.h>\n#include <math.h>\nint main(void){long long a,b;scanf(\"%lld %lld\",&a,&b);printf(\"%lld\\n\",a+b+(long long)sqrt(0.0));}",
			"cpp17":   cppSum,
			"cpp20":   cppSum,
			"cpp23":   cppSum,
			"python3": pythonSum,
			"pypy3":   pythonSum,
			"java":    "import java.util.Scanner;\npublic class Main { public static void main(String[] args) { Scanner s = new Scanner(System.in); System.out.println(s.nextLong() + s.nextLong()); } }",
			"go":      "package main\nimport \"fmt\"\nfunc main() { var a, b int64; fmt.Scan(&a, &b); fmt.Println(a + b) }",
			"rust":    "use std::io::Read;\nfn main() { let mut s = String::new(); std::io::stdin().read_to_string(&mut s).unwrap(); let n: i64 = s.split_whitespace().map(|x| x.parse::<i64>().unwrap()).sum(); println!(\"{n}\"); }",
			"node":    "const [a, b] = require(\"fs\").readFileSync(0, \"utf8\").trim().split(/\\s+/).map(BigInt);\nconsole.log((a + b).toString());",
		}
		for _, id := range cfg.Order {
			t.Run(id, func(t *testing.T) {
				t.Parallel()
				source, ok := sources[id]
				if !ok {
					t.Fatalf("no test program for configuration %q", id)
				}
				executable, err := sb.Compile(ctx, []byte(source), id, nil)
				if err != nil {
					t.Fatalf("failed to compile: %v", err)
				}
				result, err := sb.Test(ctx, executable, id, []byte("2000000000 3000000000\n"), 5000, 256)
				if err != nil {
					t.Fatalf("failed to run: %v", err)
				}
				if result.Status != sandbox.StatusOK || strings.TrimSpace(string(result.Stdout)) != "5000000000" {
					t.Fatalf("got %s with output %q (stderr %q)", result.Status, result.Stdout, result.Stderr)
				}
			})
		}
	})

	t.Run("compile latex booklet", func(t *testing.T) {
		// Uncompressed PDF objects keep the font dictionaries readable below.
		texSource := `\pdfobjcompresslevel=0 \pdfcompresslevel=0
\documentclass[11pt,a4paper,oneside]{article}
\usepackage[T2A]{fontenc}
\usepackage[utf8]{inputenc}
\usepackage[russian,english]{babel}
\usepackage{amsmath,amssymb}
\usepackage{olymp}
\contest{Test Contest}{Org}{2026}
\begin{document}
\begin{problem}{A. Test Problem}{stdin}{stdout}{1.0 сек}{256 МБ}
Test legend. Условие задачи на русском языке.
\end{problem}
\end{document}`
		pdfBytes, err := sb.CompilePDF(ctx, texSource)
		if err != nil {
			t.Fatalf("failed to compile PDF booklet in sandbox: %v", err)
		}
		if len(pdfBytes) < 4 || !strings.HasPrefix(string(pdfBytes[:4]), "%PDF") {
			t.Fatalf("expected valid PDF header, got %q", string(pdfBytes[:4]))
		}
		// Cyrillic text uses outline fonts instead of falling back to bitmap fonts.
		if strings.Contains(string(pdfBytes), "/Type3") {
			t.Fatal("booklet uses bitmap Type 3 fonts")
		}
	})
}

const (
	cppSum    = "#include <iostream>\nint main() { long long a, b; std::cin >> a >> b; std::cout << a + b << std::endl; }"
	pythonSum = "print(sum(map(int, input().split())))"
)

func getLanguageKey(filename string) (string, error) {
	ext := filepath.Ext(filename)
	switch strings.ToLower(ext) {
	case ".cpp", ".cc", ".cxx":
		return "cpp17", nil
	case ".py":
		return "python3", nil
	case ".go":
		return "go", nil
	case ".java":
		return "java", nil
	default:
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}
}

func expectedVerdict(filename string) (string, bool) {
	lower := strings.ToLower(filename)
	ext := filepath.Ext(lower)
	nameWithoutExt := strings.TrimSuffix(lower, ext)

	if strings.HasPrefix(nameWithoutExt, "ok") {
		return "OK", true
	}
	if strings.HasPrefix(nameWithoutExt, "wa") {
		return "WA", true
	}
	if strings.HasPrefix(nameWithoutExt, "tl") {
		return "TLE", true
	}
	if strings.HasPrefix(nameWithoutExt, "ml") {
		return "MLE", true
	}
	if strings.HasPrefix(nameWithoutExt, "re") {
		return "RE", true
	}
	if strings.HasPrefix(nameWithoutExt, "ce") {
		return "CE", true
	}
	if strings.HasPrefix(nameWithoutExt, "pe") {
		return "PE", true
	}
	if strings.HasPrefix(nameWithoutExt, "ole") {
		return "OLE", true
	}
	return "", false
}
