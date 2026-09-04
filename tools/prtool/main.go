package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("git repository root not found")
}

func runCommand(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%s %s failed: %w\nStderr: %s\nStdout: %s", name, strings.Join(args, " "), err, stderr.String(), stdout.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

func runCommandPiped(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]
	switch subcommand {
	case "create":
		handleCreate(os.Args[2:])
	case "review-prompt":
		handleReviewPrompt(os.Args[2:])
	case "review-comment":
		handleReviewComment(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: prtool <subcommand> [flags]

Subcommands:
  create           Validate precommit, push branch, and create a PR via gh
  review-prompt    Generate an adversarial code review prompt for Reviewer Agent
  review-comment   Post a structured review comment to a GitHub PR via gh pr review

Run 'prtool <subcommand> -h' for subcommand options.
`)
}

func handleCreate(args []string) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	titleFlag := fs.String("title", "", "PR title (must follow Conventional Commits and <= 65 chars)")
	taskFlag := fs.String("task", "", "Task ID (e.g. TASK-002). Defaults to parsing from branch name")
	bodyFlag := fs.String("body", "", "Additional custom description to append to PR template")
	baseFlag := fs.String("base", "main", "Target base branch")
	skipPrecommit := fs.Bool("skip-precommit", false, "Skip running 'task precommit'")
	dryRun := fs.Bool("dry-run", false, "Output generated PR metadata without pushing or creating PR")
	draft := fs.Bool("draft", false, "Create PR as a draft")

	fs.Parse(args)

	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// 1. Check current branch
	branch, err := runCommand(repoRoot, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current git branch: %v\n", err)
		os.Exit(1)
	}
	if branch == *baseFlag {
		fmt.Fprintf(os.Stderr, "Error: cannot create PR from base branch '%s'. Create a feature branch first.\n", *baseFlag)
		os.Exit(1)
	}

	// 2. Identify task ID
	taskID := *taskFlag
	if taskID == "" {
		taskID = ExtractTaskID(branch)
	}
	if taskID == "" {
		fmt.Fprintf(os.Stderr, "Error: every PR must be bound to a task in .tasks/. Could not extract task ID from branch '%s'. Use -task TASK-XXX\n", branch)
		os.Exit(1)
	}

	// 3. Locate and parse task file
	relTaskPath, err := FindTaskFile(repoRoot, taskID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	taskMeta, err := ParseTaskMeta(filepath.Join(repoRoot, relTaskPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing task %s: %v\n", relTaskPath, err)
		os.Exit(1)
	}

	// 4. Determine PR title
	prTitle := *titleFlag
	if prTitle == "" {
		// Try last commit subject
		lastCommit, err := runCommand(repoRoot, "git", "log", "-1", "--pretty=%s")
		if err == nil && ValidatePRTitle(lastCommit) == nil {
			prTitle = lastCommit
		}
	}
	if prTitle == "" {
		fmt.Fprintf(os.Stderr, "Error: -title is required and must follow Conventional Commits (<= 65 chars).\nExample: -title=\"%s: %s\"\n", taskMeta.Type, taskMeta.Title)
		os.Exit(1)
	}
	if err := ValidatePRTitle(prTitle); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// 5. Pre-commit check
	if !*skipPrecommit {
		fmt.Println("🔍 Running pre-commit validation ('task precommit')...")
		if err := runCommandPiped(repoRoot, "task", "precommit"); err != nil {
			fmt.Fprintf(os.Stderr, "\n❌ Pre-commit validation failed. Fix the issues before opening a PR.\n")
			os.Exit(1)
		}
		fmt.Println("✅ Pre-commit validation passed cleanly.")
	}

	// 6. Render PR body from template
	templatePath := filepath.Join(repoRoot, ".github", "pull_request_template.md")
	templateBytes, err := os.ReadFile(templatePath)
	var templateContent string
	if err == nil {
		templateContent = string(templateBytes)
	} else {
		templateContent = "## Task\nResolves / Task: <!-- e.g. .tasks/TASK-002-pr-automation-and-review-flow.md -->\n\n## Description\n<!-- Concise summary of changes and non-obvious rationale. Keep focused. -->\n"
	}
	prBody := RenderPRBody(templateContent, taskMeta, *bodyFlag)

	if *dryRun {
		fmt.Printf("\n--- [DRY-RUN] PR Preview ---\n")
		fmt.Printf("Base:   %s\n", *baseFlag)
		fmt.Printf("Head:   %s\n", branch)
		fmt.Printf("Title:  %s\n", prTitle)
		fmt.Printf("Task:   %s (%s)\n", taskMeta.ID, taskMeta.FilePath)
		fmt.Printf("Draft:  %t\n", *draft)
		fmt.Printf("Body:\n%s\n----------------------------\n", prBody)
		return
	}

	// 7. Push branch
	fmt.Printf("🚀 Pushing branch '%s' to origin...\n", branch)
	if err := runCommandPiped(repoRoot, "git", "push", "-u", "origin", branch); err != nil {
		fmt.Fprintf(os.Stderr, "Error pushing branch: %v\n", err)
		os.Exit(1)
	}

	// 8. Create PR via gh
	fmt.Println("📋 Creating pull request via 'gh pr create'...")
	ghArgs := []string{"pr", "create", "--base", *baseFlag, "--head", branch, "--title", prTitle, "--body", prBody}
	if *draft {
		ghArgs = append(ghArgs, "--draft")
	}

	prURL, err := runCommand(repoRoot, "gh", ghArgs...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating PR via gh: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Pull request successfully created:\n%s\n", prURL)
}

func handleReviewPrompt(args []string) {
	fs := flag.NewFlagSet("review-prompt", flag.ExitOnError)
	baseFlag := fs.String("base", "origin/main", "Base branch to diff against")
	taskFlag := fs.String("task", "", "Task ID (defaults to branch name)")
	outFlag := fs.String("out", "", "Write prompt to file instead of stdout")

	fs.Parse(args)

	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	branch, _ := runCommand(repoRoot, "git", "rev-parse", "--abbrev-ref", "HEAD")
	taskID := *taskFlag
	if taskID == "" {
		taskID = ExtractTaskID(branch)
	}

	var taskContent string
	var taskMeta *TaskMeta
	if taskID != "" {
		taskPath, err := FindTaskFile(repoRoot, taskID)
		if err == nil {
			b, _ := os.ReadFile(filepath.Join(repoRoot, taskPath))
			taskContent = string(b)
			taskMeta, _ = ParseTaskMeta(filepath.Join(repoRoot, taskPath))
		}
	}

	diff, err := runCommand(repoRoot, "git", "diff", fmt.Sprintf("%s...HEAD", *baseFlag))
	if err != nil {
		// Try fallback to local base branch
		diff, _ = runCommand(repoRoot, "git", "diff", "main...HEAD")
	}

	agentsMD, _ := os.ReadFile(filepath.Join(repoRoot, "AGENTS.md"))

	var prompt strings.Builder
	prompt.WriteString("# Adversarial Code Review Request\n\n")
	prompt.WriteString("You are acting as an objective, rigorous Code Reviewer.\n")
	prompt.WriteString("Your goal is to verify that the implementation satisfies all acceptance criteria, adheres strictly to AGENTS.md, introduces no unnecessary or non-surgical changes, and contains adequate tests without edge case bugs.\n\n")

	if taskMeta != nil {
		prompt.WriteString(fmt.Sprintf("## Target Task: %s — %s\n", taskMeta.ID, taskMeta.Title))
		prompt.WriteString("### Task Specification:\n```markdown\n")
		prompt.WriteString(taskContent)
		prompt.WriteString("\n```\n\n")
	}

	prompt.WriteString("## Project Guidelines & Rules (AGENTS.md)\n```markdown\n")
	prompt.WriteString(string(agentsMD))
	prompt.WriteString("\n```\n\n")

	prompt.WriteString("## Git Diff (`" + *baseFlag + "...HEAD`)\n```diff\n")
	prompt.WriteString(diff)
	prompt.WriteString("\n```\n\n")

	prompt.WriteString(`## Review Instructions & Output Format
Please conduct an adversarial audit and provide your findings in this exact format:

### 1. Executive Summary
- **Verdict**: [APPROVE | REQUEST_CHANGES]
- **Key summary**: Brief 2-3 sentence overview.

### 2. Acceptance Criteria Verification
Table of each acceptance criteria item from the task:
| Criteria | Status | Notes |
| :--- | :--- | :--- |

### 3. AGENTS.md Compliance Checklist
- [ ] Surgical Changes (only requested code touched, no dead code, no formatting pollution)
- [ ] No symlinks created
- [ ] Frontend (if applicable): next.config.mjs & next-env.d.ts untouched
- [ ] Frontend (if applicable): No env fallback values in code
- [ ] Frontend (if applicable): No Server Actions ('use server')
- [ ] Backend (if applicable): slog exclusively used for logging
- [ ] Precommit and conventional commit rules satisfied

### 4. Detailed Findings & Actionable Feedback
If REQUEST_CHANGES, provide concrete file:line pointers and exact code fixes. If APPROVE, note any non-blocking suggestions.
`)

	if *outFlag != "" {
		if err := os.WriteFile(*outFlag, []byte(prompt.String()), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write prompt to %s: %v\n", *outFlag, err)
			os.Exit(1)
		}
		fmt.Printf("✅ Review prompt written to %s\n", *outFlag)
	} else {
		fmt.Println(prompt.String())
	}
}

func handleReviewComment(args []string) {
	fs := flag.NewFlagSet("review-comment", flag.ExitOnError)
	prFlag := fs.String("pr", "", "PR number or URL (optional, defaults to current branch PR)")
	fileFlag := fs.String("file", "", "Markdown file containing review comments")
	bodyFlag := fs.String("body", "", "Direct string comment content")

	fs.Parse(args)

	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var commentBody string
	if *fileFlag != "" {
		content, err := os.ReadFile(*fileFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", *fileFlag, err)
			os.Exit(1)
		}
		commentBody = string(content)
	} else if *bodyFlag != "" {
		commentBody = *bodyFlag
	} else {
		fmt.Fprintf(os.Stderr, "Error: either -file or -body must be specified\n")
		os.Exit(1)
	}

	ghArgs := []string{"pr", "review"}
	if *prFlag != "" {
		ghArgs = append(ghArgs, *prFlag)
	}
	ghArgs = append(ghArgs, "--comment", "-b", commentBody)

	fmt.Println("💬 Posting review comment via 'gh pr review'...")
	out, err := runCommand(repoRoot, "gh", ghArgs...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error posting review comment: %v\n", err)
		os.Exit(1)
	}
	if out != "" {
		fmt.Println(out)
	}
	fmt.Println("✅ Review comment posted successfully.")
}
