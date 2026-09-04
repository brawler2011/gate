---
id: TASK-011
title: "Implement submission programming language autodetection"
status: todo
type: feat
description: "Autodetect source code programming language (Python, C++, Go) in submission editor."
priority: normal
created_at: 2025-05-11
tags:
  - frontend
  - submissions
  - editor
---

# TASK-011: Implement submission programming language autodetection

## Context
Imported from YouGile TID-41 ("Code language autodetect based on language: python, cpp, golang, auto"). When participants paste or upload solution code into CreateSubmissionForm.tsx, requiring manual language selection leads to mistakes (e.g. submitting C++ code under Python compiler). An autodetection mechanism should recognize Python, C++, and Go syntax while preserving manual override.

## Acceptance Criteria
- [ ] Implement lightweight language heuristic/detector supporting Python, C++, Go, and Auto mode
- [ ] Integrate detector with CreateSubmissionForm.tsx code editor on paste and file upload
- [ ] Automatically update language dropdown when confidence threshold is met, allowing manual override
- [ ] Unit tests testing language detection heuristics on diverse source samples
- [ ] task precommit:fe passes cleanly

## Implementation Notes
Ensure autodetection runs asynchronously or debounced to prevent UI lag on large code pastes.
