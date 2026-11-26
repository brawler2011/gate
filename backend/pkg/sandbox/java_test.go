package sandbox

import "testing"

func TestJavaClass(t *testing.T) {
	for name, tc := range map[string]struct{ source, want string }{
		"named after the solution": {"import java.util.*;\npublic class Ideal_gk {\n public static void main(String[] a) {}\n}\n", "Ideal_gk"},
		"modifiers and generics":   {"package p;\n@SuppressWarnings(\"x\")\npublic final class Sol<T> {}", "Sol"},
		"helper class first":       {"class Pair { public int a; }\npublic class Solver { class Inner {} }", "Solver"},
		"public nested only":       {"class A { public static class B {} }", defaultClass},
		"no public class":          {"class Solution { public static void main(String[] a) {} }", defaultClass},
		"comments and strings":     {"// public class Fake1\n/* public class Fake2 */\nclass X { String s = \"} public class Fake3 {\"; char c = '}'; String t = \"\"\"\n}\npublic class Fake4\n\"\"\"; }\npublic class Real {}", "Real"},
		"public record":            {"public record Point(int x) {}", "Point"},
		"unterminated comment":     {"public class A /* {", "A"},
		"empty":                    {"", defaultClass},
	} {
		t.Run(name, func(t *testing.T) {
			if got := javaClass([]byte(tc.source)); got != tc.want {
				t.Fatalf("javaClass = %q, want %q", got, tc.want)
			}
		})
	}
}
