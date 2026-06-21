package sandbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gate149/gate/backend/pkg/formats/gfmt"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestIntegrationSandbox(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Spin up go-judge container using testcontainers-go and the local Dockerfile
	t.Log("Starting go-judge container...")
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    ".",
			Dockerfile: "Dockerfile",
			KeepImage:  true,
		},
		ExposedPorts: []string{"5051/tcp"},
		WaitingFor:   wait.ForListeningPort("5051/tcp"),
		Privileged:   true,
		Cmd:          []string{"-enable-grpc", "-grpc-addr=0.0.0.0:5051"},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start go-judge container: %v", err)
	}
	defer func() {
		if err := container.Terminate(ctx); err != nil {
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

	// Create gRPC client
	client, err := NewClient(grpcAddr)
	if err != nil {
		t.Fatalf("Failed to create sandbox client: %v", err)
	}
	defer client.Close()

	// Load languages config
	cfg, err := LoadConfig("../../languages.yaml")
	if err != nil {
		t.Fatalf("Failed to load languages config: %v", err)
	}

	sb := NewSandbox(client, cfg)

	// Dynamically scan testdata/gfmt/ for packages
	gfmtRoot := "../testdata/gfmt"
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

				// Read all files in lib/
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

				// Compile checker if it exists
				var checker Executable
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

				// Compile interactor if it exists
				var interactor Executable
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

				// Compile generators if they exist
				generators := make(map[string]Executable)
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

				// Parse expected test cases
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

				// Helper to resolve test case input
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

				// Discover solutions
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

				// Compile reference solver
				refCode, err := pkg.GetSolution(referenceSol.Filename)
				if err != nil {
					t.Fatalf("failed to read reference solution %s: %v", referenceSol.Filename, err)
				}
				refSolID, err := sb.Compile(ctx, refCode, referenceSol.Lang, nil)
				if err != nil {
					t.Fatalf("failed to compile reference solution %s: %v", referenceSol.Filename, err)
				}

				// Generate test case inputs and reference answers
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
						if solResult.Status != StatusOK {
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

				// Test all solutions
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

						finalStatus := StatusOK
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
								if interactRes.Status != StatusOK {
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
								if runRes.Status != StatusOK {
									finalStatus = runRes.Status
									lastStderr = runRes.Stderr
									lastExitStatus = runRes.ExitStatus
									break
								}
								checkRes, err := sb.Check(ctx, checker, tcd.Input, runRes.Stdout, tcd.Answer)
								if err != nil {
									t.Fatalf("checker failed: %v", err)
								}
								if checkRes.Status != StatusOK {
									finalStatus = checkRes.Status
									break
								}
							}
						}

						if string(finalStatus) != sol.Expected {
							if sol.Expected == "MLE" && finalStatus == StatusRE {
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
}

func getLanguageKey(filename string) (string, error) {
	ext := filepath.Ext(filename)
	switch strings.ToLower(ext) {
	case ".cpp", ".cc", ".cxx":
		return "cpp", nil
	case ".py":
		return "python", nil
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

func TestExtractScore(t *testing.T) {
	tests := []struct {
		input    string
		expected *float64
	}{
		{
			input:    "points 0.833 Your solution is 83.3% optimal",
			expected: func(val float64) *float64 { return &val }(0.833),
		},
		{
			input:    "points 0.5\nSome other message",
			expected: func(val float64) *float64 { return &val }(0.5),
		},
		{
			input:    "0.833",
			expected: func(val float64) *float64 { return &val }(0.833),
		},
		{
			input:    "0.833 Your solution is 83.3% optimal",
			expected: func(val float64) *float64 { return &val }(0.833),
		},
		{
			input:    "Wrong Answer on test 1",
			expected: nil,
		},
	}

	for _, tc := range tests {
		res := parseScore(tc.input)
		if tc.expected == nil {
			if res != nil {
				t.Errorf("expected nil for input %q, got %f", tc.input, *res)
			}
		} else {
			if res == nil {
				t.Errorf("expected %f for input %q, got nil", *tc.expected, tc.input)
			} else if *res != *tc.expected {
				t.Errorf("expected %f for input %q, got %f", *tc.expected, tc.input, *res)
			}
		}
	}
}
