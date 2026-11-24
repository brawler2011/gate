//go:build integration

package sandbox_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/bytesize"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/syslib"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/brawler2011/gate/backend/tests/judgeimage"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gopkg.in/yaml.v3"
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
		ExposedPorts: []string{"5050/tcp"},
		WaitingFor:   wait.ForHTTP("/version").WithPort("5050/tcp"),
		Privileged:   true,
		Cmd:          []string{"-http-addr=0.0.0.0:5050", "-no-fallback"},
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

	port, err := container.MappedPort(ctx, "5050")
	if err != nil {
		t.Fatalf("Failed to get container mapped port: %v", err)
	}

	base := fmt.Sprintf("http://%s:%s", ip, port.Port())
	client, err := sandbox.NewClient(base, &http.Client{})
	if err != nil {
		t.Fatalf("Failed to create sandbox client: %v", err)
	}
	files, err := sandbox.NewFileTransport(base, &http.Client{Timeout: time.Minute})
	if err != nil {
		t.Fatalf("Failed to create sandbox file transport: %v", err)
	}

	cfg, err := sandbox.LoadConfig("../../../../judge/langs.toml")
	if err != nil {
		t.Fatalf("Failed to load languages config: %v", err)
	}

	sb := sandbox.NewSandbox(client, cfg)
	r := runner{sb: sb, files: files}
	limits := dbfixtures.BuildLimits(t)

	for i := 0; i < 30; i++ {
		probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		_, err := sb.CompileWithLimits(probeCtx, []byte("int main(){return 0;}"), "cpp17", nil, limits.Compile)
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

	t.Run("solution files", func(t *testing.T) {
		input := r.upload(t, []byte("1 2\n"))
		both := sandbox.Streams{Input: "input.txt", Output: "output.txt"}
		for _, c := range []struct {
			name, source string
			streams      sandbox.Streams
			want         string
		}{
			{"files", "open('output.txt', 'w').write(open('input.txt').read())", both, "1 2\n"},
			// Standard streams are empty and unread, so the checker gets an empty output.
			{"standard streams", "import sys\nsys.stdout.write(sys.stdin.read() or 'stdin was empty')", both, ""},
			{"input file only", "import sys\nsys.stdout.write(open('input.txt').read())", sandbox.Streams{Input: "input.txt"}, "1 2\n"},
			{"output file only", "import sys\nopen('output.txt', 'w').write(sys.stdin.read())", sandbox.Streams{Output: "output.txt"}, "1 2\n"},
		} {
			t.Run(c.name, func(t *testing.T) {
				exe, err := sb.CompileWithLimits(ctx, []byte(c.source), "python3", nil, limits.Compile)
				if err != nil {
					t.Fatalf("compile: %v", err)
				}
				res, err := sb.RunSolution(ctx, exe, "python3", input, c.streams, solutionLimits(limits, 2000, 256))
				if err != nil {
					t.Fatalf("run: %v", err)
				}
				defer r.release(res.OutputID)
				if res.Status != sandbox.StatusOK {
					t.Fatalf("status %s: %s", res.Status, res.Stderr)
				}
				var output bytes.Buffer
				if _, err := files.Download(ctx, res.OutputID, &output, 1<<20); err != nil {
					t.Fatalf("download output: %v", err)
				}
				if output.String() != c.want {
					t.Fatalf("output %q, want %q", output.String(), c.want)
				}
			})
		}
		t.Run("output file over the limit", func(t *testing.T) {
			exe, err := sb.CompileWithLimits(ctx, []byte("open('output.txt', 'w').write('x' * (11 << 20))"), "python3", nil, limits.Compile)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			over := solutionLimits(limits, 2000, 256)
			over.Output = 10 * bytesize.MiB
			res, err := sb.RunSolution(ctx, exe, "python3", input, both, over)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			r.release(res.OutputID)
			if res.Status != sandbox.StatusOLE {
				t.Fatalf("status %s, want OLE", res.Status)
			}
		})
	})

	t.Run("packages", func(t *testing.T) {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			pkgName := entry.Name()
			pkgPath := filepath.Join(gfmtRoot, pkgName)

			t.Run(pkgName, func(t *testing.T) {
				t.Parallel()
				pkg, err := openAuthorPackage(pkgPath)
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

				var checker program
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
							exe, err := sb.CompileWithLimits(ctx, code, langKey, deps, limits.Compile)
							if err != nil {
								t.Fatalf("failed to compile checker %s: %v", f.Name(), err)
							}
							checker = program{exe, langKey}
							break
						}
					}
				}

				var interactor program
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
							exe, err := sb.CompileWithLimits(ctx, code, langKey, deps, limits.Compile)
							if err != nil {
								t.Fatalf("failed to compile interactor %s: %v", f.Name(), err)
							}
							interactor = program{exe, langKey}
							break
						}
					}
				}

				generators := make(map[string]program)
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
							exe, err := sb.CompileWithLimits(ctx, code, langKey, deps, limits.Compile)
							if err != nil {
								t.Fatalf("failed to compile generator %s: %v", f.Name(), err)
							}
							nameWithoutExt := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))
							generators[nameWithoutExt] = program{exe, langKey}
						}
					}
				}

				type TestCase struct {
					Name     string
					Input    string
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
							Input:    tc.Input,
							Generate: tc.Generate,
						})
					}
				}

				getInput := func(tc TestCase) ([]byte, error) {
					if tc.Generate == "" {
						return pkg.testInput(tc.Input)
					}
					parts := strings.Fields(tc.Generate)
					if len(parts) == 0 {
						return nil, fmt.Errorf("empty generate command")
					}
					genName := parts[0]
					args := parts[1:]
					gen, ok := generators[genName]
					if !ok {
						return nil, fmt.Errorf("generator not found: %s", genName)
					}
					res, stdout := r.run(t, gen, sandbox.File{}, args, limits.Auxiliary)
					if res.Status != sandbox.StatusOK {
						return nil, fmt.Errorf("generator %s: %s: %s", genName, res.Status, res.Stderr)
					}
					return stdout, nil
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

				refCode, err := pkg.solution(referenceSol.Filename)
				if err != nil {
					t.Fatalf("failed to read reference solution %s: %v", referenceSol.Filename, err)
				}
				refExe, err := sb.CompileWithLimits(ctx, refCode, referenceSol.Lang, nil, limits.Compile)
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
						solResult, stdout := r.run(t, program{refExe, referenceSol.Lang}, r.upload(t, input), nil, solutionLimits(limits, timeLimit, memLimit))
						if solResult.Status != sandbox.StatusOK {
							t.Fatalf("reference solution did not return OK on test %s: status=%s, stderr=%s",
								tc.Name, solResult.Status, string(solResult.Stderr))
						}
						answer = stdout
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
						solCode, err := pkg.solution(sol.Filename)
						if err != nil {
							t.Fatalf("failed to read solution %s: %v", sol.Filename, err)
						}

						solExe, err := sb.CompileWithLimits(ctx, solCode, sol.Lang, nil, limits.Compile)
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
								interactRes, err := sb.InteractFiles(ctx, solExe, sol.Lang, interactor.exe, interactor.lang, r.upload(t, tcd.Input), sandbox.File{}, solutionLimits(limits, timeLimit, memLimit), limits.Auxiliary)
								if err != nil {
									t.Fatalf("interact failed: %v", err)
								}
								r.release(interactRes.OutputID)
								if interactRes.Status != sandbox.StatusOK {
									finalStatus = interactRes.Status
									lastStderr = interactRes.SolutionResult.Stderr
									lastExitStatus = interactRes.SolutionResult.ExitStatus
									break
								}
							} else {
								runRes, stdout := r.run(t, program{solExe, sol.Lang}, r.upload(t, tcd.Input), nil, solutionLimits(limits, timeLimit, memLimit))
								if runRes.Status != sandbox.StatusOK {
									finalStatus = runRes.Status
									lastStderr = runRes.Stderr
									lastExitStatus = runRes.ExitStatus
									break
								}
								checkRes, err := sb.CheckFiles(ctx, checker.exe, checker.lang, r.upload(t, tcd.Input), r.upload(t, stdout), r.upload(t, tcd.Answer), limits.Auxiliary)
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
		executable, err := sb.CompileWithLimits(ctx, []byte(source), "java", nil, limits.Compile)
		if err != nil {
			t.Fatalf("failed to compile: %v", err)
		}
		result, stdout := r.run(t, program{executable, "java"}, sandbox.File{}, nil, solutionLimits(limits, 5000, 256))
		if result.Status != sandbox.StatusOK || strings.TrimSpace(string(stdout)) != "2.5" {
			t.Fatalf("got %s with output %q (stderr %q)", result.Status, stdout, result.Stderr)
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
				executable, err := sb.CompileWithLimits(ctx, []byte(source), id, nil, limits.Compile)
				if err != nil {
					t.Fatalf("failed to compile: %v", err)
				}
				result, stdout := r.run(t, program{executable, id}, r.upload(t, []byte("2000000000 3000000000\n")), nil, solutionLimits(limits, 5000, 256))
				if result.Status != sandbox.StatusOK || strings.TrimSpace(string(stdout)) != "5000000000" {
					t.Fatalf("got %s with output %q (stderr %q)", result.Status, stdout, result.Stderr)
				}
			})
		}
	})

	// JSON would turn these bytes into U+FFFD, so the source must travel through the file store.
	t.Run("non-UTF-8 source", func(t *testing.T) {
		cp1251 := "\xcf\xf0\xe8\xe2\xe5\xf2"
		executable, err := sb.CompileWithLimits(ctx, []byte("#include <cstdio>\nint main(){fputs(\""+cp1251+"\", stdout);}"), "cpp17", nil, limits.Compile)
		if err != nil {
			t.Fatalf("failed to compile: %v", err)
		}
		result, stdout := r.run(t, program{executable, "cpp17"}, sandbox.File{}, nil, solutionLimits(limits, 5000, 256))
		if result.Status != sandbox.StatusOK || string(stdout) != cp1251 {
			t.Fatalf("got %s with output %q (stderr %q)", result.Status, stdout, result.Stderr)
		}
	})
}

type program struct {
	exe  sandbox.Executable
	lang string
}

func solutionLimits(limits model.BuildLimits, timeMs, memoryMb int) sandbox.FileRunLimits {
	return limits.Solution(time.Duration(timeMs)*time.Millisecond, bytesize.Size(memoryMb)*bytesize.MiB)
}

// runner feeds in-memory test data through the file store, as judging does with package files.
type runner struct {
	sb    *sandbox.Sandbox
	files *sandbox.FileTransport
}

func (r runner) upload(t *testing.T, data []byte) sandbox.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "input")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		t.Fatal(err)
	}
	id, err := r.files.Upload(context.Background(), f, 1<<30)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	t.Cleanup(func() { r.release(id) })
	return sandbox.FileID(id)
}

func (r runner) release(id string) {
	if id != "" {
		_ = r.files.Delete(context.Background(), id)
	}
}

func (r runner) run(t *testing.T, p program, input sandbox.File, args []string, limits sandbox.FileRunLimits) (*sandbox.FileRunResult, []byte) {
	t.Helper()
	res, err := r.sb.RunFiles(context.Background(), p.exe, p.lang, input, args, nil, limits)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	defer r.release(res.OutputID)
	var stdout bytes.Buffer
	if res.OutputID != "" {
		if _, err := r.files.Download(context.Background(), res.OutputID, &stdout, int64(limits.Output)); err != nil {
			t.Fatalf("download output: %v", err)
		}
	}
	return res, stdout.Bytes()
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

type authorPackage struct {
	Path    string
	Problem *gfmt.Problem
}

func openAuthorPackage(dir string) (*authorPackage, error) {
	data, err := os.ReadFile(filepath.Join(dir, "problem.yaml"))
	if err != nil {
		return nil, err
	}
	var p gfmt.Problem
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &authorPackage{Path: dir, Problem: &p}, nil
}

func (p *authorPackage) testInput(name string) ([]byte, error) {
	for _, candidate := range gfmt.TestInputNames(name) {
		if data, err := os.ReadFile(filepath.Join(p.Path, gfmt.TestFolder, candidate)); err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("test input file not found for %s", name)
}

func (p *authorPackage) solution(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(p.Path, gfmt.RoleFolder("solution"), name))
}
