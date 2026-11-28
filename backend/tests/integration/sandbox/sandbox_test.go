//go:build integration

package sandbox_test

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/bytesize"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/brawler2011/gate/backend/tests/judgeimage"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var variant = flag.String("variant", sandbox.VariantMinimal, "judge image variant to run")

func TestIntegrationSandbox(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("Starting go-judge container...")
	ref := judgeimage.Ref
	if *variant == sandbox.VariantFull {
		ref = judgeimage.FullRef
	}
	image, err := ref()
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

	judge, err := sandbox.ReadImage(ctx, client)
	if err != nil {
		t.Fatalf("Failed to read judge image manifest: %v", err)
	}
	cfg := judge.Config
	if len(cfg.Missing) > 0 && *variant == sandbox.VariantFull {
		t.Fatalf("full image lacks %v", cfg.Missing)
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

	// A non-ASCII class name also checks that javac accepts its file name.
	t.Run("java class named after the solution", func(t *testing.T) {
		source := "public class Решение { public static void main(String[] args) { System.out.println(42); } }"
		executable, err := sb.CompileWithLimits(ctx, []byte(source), "java", nil, limits.Compile)
		if err != nil {
			t.Fatalf("failed to compile: %v", err)
		}
		result, stdout := r.run(t, program{executable, "java"}, sandbox.File{}, nil, solutionLimits(limits, 5000, 256))
		if result.Status != sandbox.StatusOK || strings.TrimSpace(string(stdout)) != "42" {
			t.Fatalf("got %s with output %q (stderr %q)", result.Status, stdout, result.Stderr)
		}
	})

	// GNU extensions such as typeof need -std=gnu++; the recursion needs far more than a default 8 MiB stack.
	t.Run("deep recursion", func(t *testing.T) {
		source := "#include <cstdio>\nint depth(int n) { volatile char pad[64]; pad[n % 64] = 1; if (n == 0) return 0; int d = depth(n - 1); return d + pad[n % 64]; }\nint main() { typeof(depth(0)) d = depth(1000000); printf(\"%d\\n\", d); }"
		executable, err := sb.CompileWithLimits(ctx, []byte(source), "cpp17", nil, limits.Compile)
		if err != nil {
			t.Fatalf("failed to compile: %v", err)
		}
		result, stdout := r.run(t, program{executable, "cpp17"}, sandbox.File{}, nil, solutionLimits(limits, 5000, 256))
		if result.Status != sandbox.StatusOK || strings.TrimSpace(string(stdout)) != "1000000" {
			t.Fatalf("got %s with output %q (stderr %q)", result.Status, stdout, result.Stderr)
		}
	})

	t.Run("java deep recursion", func(t *testing.T) {
		source := "public class Main { static int depth(int n) { return n == 0 ? 0 : 1 + depth(n - 1); } public static void main(String[] args) { System.out.println(depth(200000)); } }"
		executable, err := sb.CompileWithLimits(ctx, []byte(source), "java", nil, limits.Compile)
		if err != nil {
			t.Fatalf("failed to compile: %v", err)
		}
		result, stdout := r.run(t, program{executable, "java"}, sandbox.File{}, nil, solutionLimits(limits, 5000, 256))
		if result.Status != sandbox.StatusOK || strings.TrimSpace(string(stdout)) != "200000" {
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
			"java":    "import java.util.Scanner;\npublic class Main { record Pair(long a, long b) {} public static void main(String[] args) { Scanner s = new Scanner(System.in); Pair p = new Pair(s.nextLong(), s.nextLong()); System.out.println(p.a() + p.b()); } }",
			"go":      "package main\nimport \"fmt\"\nfunc main() { var a, b int64; fmt.Scan(&a, &b); fmt.Println(a + b) }",
			"rust":    "use std::io::Read;\nfn main() { let mut s = String::new(); std::io::stdin().read_to_string(&mut s).unwrap(); let n: i64 = s.split_whitespace().map(|x| x.parse::<i64>().unwrap()).sum(); println!(\"{n}\"); }",
			// gen is a keyword since the 2024 edition.
			"rust2021": "use std::io::Read;\nfn main() { let mut s = String::new(); std::io::stdin().read_to_string(&mut s).unwrap(); let gen: i64 = s.split_whitespace().map(|x| x.parse::<i64>().unwrap()).sum(); println!(\"{gen}\"); }",
			"node":     "const [a, b] = require(\"fs\").readFileSync(0, \"utf8\").trim().split(/\\s+/).map(BigInt);\nconsole.log((a + b).toString());",
			"kotlin":   "fun main() { val (a, b) = readln().split(\" \").map { it.toLong() }; println(a + b) }",
			"scala":    "object Main { def main(args: Array[String]): Unit = { val Array(a, b) = scala.io.StdIn.readLine().split(\" \").map(_.toLong); println(a + b) } }",
			"csharp":   "using System; using System.Linq; var v = Console.ReadLine()!.Split(' ').Select(long.Parse).ToArray(); Console.WriteLine(v[0] + v[1]);",
			"d":        "import std.stdio; void main() { long a, b; readf(\" %d %d\", a, b); writeln(a + b); }",
			"pascal":   "program sum; var a, b: int64; begin read(a, b); writeln(a + b); end.",
			"haskell":  "main :: IO ()\nmain = interact $ \\s -> show (sum (map read (words s) :: [Integer])) ++ \"\\n\"",
			"ocaml":    "let () = Scanf.scanf \" %Ld %Ld\" (fun a b -> print_endline (Int64.to_string (Int64.add a b)))",
			"ruby":     "puts gets.split.map(&:to_i).sum",
			"php":      "<?php [$a, $b] = array_map('intval', explode(' ', trim(fgets(STDIN)))); echo $a + $b, \"\\n\";",
			"perl":     "my ($a, $b) = split ' ', <STDIN>; print $a + $b, \"\\n\";",
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
