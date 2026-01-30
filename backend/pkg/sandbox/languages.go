package sandbox

// LanguageConfig defines compilation and execution settings for a language
type LanguageConfig struct {
	Name             string
	Extension        string
	CompileCommand   []string // command template: will replace placeholders
	ExecuteCommand   []string // command template for execution
	CompileLimits    ResourceLimits
	ExecuteLimits    ResourceLimits
	NeedsCompilation bool
	CompilerEnv      []string // environment variables for compilation
	ExecuteEnv       []string // environment variables for execution
}

// DefaultLanguages returns a map of supported language configurations
func DefaultLanguages() map[string]LanguageConfig {
	return map[string]LanguageConfig{
		"cpp11": {
			Name:             "C++11",
			Extension:        ".cpp",
			CompileCommand:   []string{"/usr/bin/g++", "-std=c++11", "-O2", "-Wall", "-o", "{output}", "{source}"},
			ExecuteCommand:   []string{"./{executable}"},
			NeedsCompilation: true,
			CompileLimits: ResourceLimits{
				CPUTimeMs: 30000, // 30 seconds
				MemoryMB:  512,
				ProcLimit: 50,
				StackMB:   256,
			},
			ExecuteLimits: ResourceLimits{
				CPUTimeMs: 5000, // 5 seconds default
				MemoryMB:  256,
				ProcLimit: 1,
				StackMB:   256,
			},
			CompilerEnv: []string{"PATH=/usr/bin:/bin"},
			ExecuteEnv:  []string{"PATH=/usr/bin:/bin"},
		},
		"cpp14": {
			Name:             "C++14",
			Extension:        ".cpp",
			CompileCommand:   []string{"/usr/bin/g++", "-std=c++14", "-O2", "-Wall", "-o", "{output}", "{source}"},
			ExecuteCommand:   []string{"./{executable}"},
			NeedsCompilation: true,
			CompileLimits: ResourceLimits{
				CPUTimeMs: 30000,
				MemoryMB:  512,
				ProcLimit: 50,
				StackMB:   256,
			},
			ExecuteLimits: ResourceLimits{
				CPUTimeMs: 5000,
				MemoryMB:  256,
				ProcLimit: 1,
				StackMB:   256,
			},
			CompilerEnv: []string{"PATH=/usr/bin:/bin"},
			ExecuteEnv:  []string{"PATH=/usr/bin:/bin"},
		},
		"cpp17": {
			Name:             "C++17",
			Extension:        ".cpp",
			CompileCommand:   []string{"/usr/bin/g++", "-std=c++17", "-O2", "-Wall", "-o", "{output}", "{source}"},
			ExecuteCommand:   []string{"./{executable}"},
			NeedsCompilation: true,
			CompileLimits: ResourceLimits{
				CPUTimeMs: 30000,
				MemoryMB:  512,
				ProcLimit: 50,
				StackMB:   256,
			},
			ExecuteLimits: ResourceLimits{
				CPUTimeMs: 5000,
				MemoryMB:  256,
				ProcLimit: 1,
				StackMB:   256,
			},
			CompilerEnv: []string{"PATH=/usr/bin:/bin"},
			ExecuteEnv:  []string{"PATH=/usr/bin:/bin"},
		},
		"cpp20": {
			Name:             "C++20",
			Extension:        ".cpp",
			CompileCommand:   []string{"/usr/bin/g++", "-std=c++20", "-O2", "-Wall", "-o", "{output}", "{source}"},
			ExecuteCommand:   []string{"./{executable}"},
			NeedsCompilation: true,
			CompileLimits: ResourceLimits{
				CPUTimeMs: 30000,
				MemoryMB:  512,
				ProcLimit: 50,
				StackMB:   256,
			},
			ExecuteLimits: ResourceLimits{
				CPUTimeMs: 5000,
				MemoryMB:  256,
				ProcLimit: 1,
				StackMB:   256,
			},
			CompilerEnv: []string{"PATH=/usr/bin:/bin"},
			ExecuteEnv:  []string{"PATH=/usr/bin:/bin"},
		},
		"c11": {
			Name:             "C11",
			Extension:        ".c",
			CompileCommand:   []string{"/usr/bin/gcc", "-std=c11", "-O2", "-Wall", "-o", "{output}", "{source}"},
			ExecuteCommand:   []string{"./{executable}"},
			NeedsCompilation: true,
			CompileLimits: ResourceLimits{
				CPUTimeMs: 30000,
				MemoryMB:  512,
				ProcLimit: 50,
				StackMB:   256,
			},
			ExecuteLimits: ResourceLimits{
				CPUTimeMs: 5000,
				MemoryMB:  256,
				ProcLimit: 1,
				StackMB:   256,
			},
			CompilerEnv: []string{"PATH=/usr/bin:/bin"},
			ExecuteEnv:  []string{"PATH=/usr/bin:/bin"},
		},
		"python3": {
			Name:             "Python 3",
			Extension:        ".py",
			CompileCommand:   []string{}, // no compilation needed
			ExecuteCommand:   []string{"/usr/bin/python3", "{source}"},
			NeedsCompilation: false,
			CompileLimits:    ResourceLimits{}, // not used
			ExecuteLimits: ResourceLimits{
				CPUTimeMs: 10000, // Python is slower, allow more time
				MemoryMB:  256,
				ProcLimit: 1,
				StackMB:   256,
			},
			CompilerEnv: []string{},
			ExecuteEnv:  []string{"PATH=/usr/bin:/bin", "PYTHONIOENCODING=utf-8"},
		},
		"golang": {
			Name:             "Go",
			Extension:        ".go",
			CompileCommand:   []string{"/usr/local/go/bin/go", "build", "-o", "{output}", "{source}"},
			ExecuteCommand:   []string{"./{executable}"},
			NeedsCompilation: true,
			CompileLimits: ResourceLimits{
				CPUTimeMs: 60000, // Go compilation can be slow
				MemoryMB:  1024,
				ProcLimit: 100, // Go compiler spawns many processes
				StackMB:   512,
			},
			ExecuteLimits: ResourceLimits{
				CPUTimeMs: 5000,
				MemoryMB:  512, // Go has GC overhead
				ProcLimit: 10,  // Go runtime uses multiple threads
				StackMB:   512,
			},
			CompilerEnv: []string{"PATH=/usr/local/go/bin:/usr/bin:/bin", "GOCACHE=/tmp", "HOME=/tmp"},
			ExecuteEnv:  []string{"PATH=/usr/bin:/bin"},
		},
		"java11": {
			Name:             "Java 11",
			Extension:        ".java",
			CompileCommand:   []string{"/usr/bin/javac", "-encoding", "UTF-8", "{source}"},
			ExecuteCommand:   []string{"/usr/bin/java", "-Xmx{memory}m", "-Xss256m", "{class}"},
			NeedsCompilation: true,
			CompileLimits: ResourceLimits{
				CPUTimeMs: 30000,
				MemoryMB:  512,
				ProcLimit: 50,
				StackMB:   256,
			},
			ExecuteLimits: ResourceLimits{
				CPUTimeMs: 10000, // JVM startup overhead
				MemoryMB:  512,   // JVM memory overhead
				ProcLimit: 50,    // JVM threads
				StackMB:   256,
			},
			CompilerEnv: []string{"PATH=/usr/bin:/bin"},
			ExecuteEnv:  []string{"PATH=/usr/bin:/bin"},
		},
		"java17": {
			Name:             "Java 17",
			Extension:        ".java",
			CompileCommand:   []string{"/usr/bin/javac", "-encoding", "UTF-8", "{source}"},
			ExecuteCommand:   []string{"/usr/bin/java", "-Xmx{memory}m", "-Xss256m", "{class}"},
			NeedsCompilation: true,
			CompileLimits: ResourceLimits{
				CPUTimeMs: 30000,
				MemoryMB:  512,
				ProcLimit: 50,
				StackMB:   256,
			},
			ExecuteLimits: ResourceLimits{
				CPUTimeMs: 10000,
				MemoryMB:  512,
				ProcLimit: 50,
				StackMB:   256,
			},
			CompilerEnv: []string{"PATH=/usr/bin:/bin"},
			ExecuteEnv:  []string{"PATH=/usr/bin:/bin"},
		},
	}
}

// GetLanguageConfig returns the configuration for a given language
func GetLanguageConfig(language string) (LanguageConfig, bool) {
	langs := DefaultLanguages()
	config, ok := langs[language]
	return config, ok
}

// NormalizeLanguageName maps common language names to standard identifiers
func NormalizeLanguageName(name string) string {
	mapping := map[string]string{
		"c++11":  "cpp11",
		"c++14":  "cpp14",
		"c++17":  "cpp17",
		"c++20":  "cpp20",
		"cpp":    "cpp17", // default to cpp17
		"c++":    "cpp17",
		"c":      "c11",
		"python": "python3",
		"py":     "python3",
		"go":     "golang",
		"java":   "java17", // default to java17
		"java11": "java11",
		"java17": "java17",
	}

	if normalized, ok := mapping[name]; ok {
		return normalized
	}
	return name
}
