package sandbox

import (
	"context"
	"fmt"
	"slices"
)

// Protocol is how a checker, interactor or validator is called and reports its verdict.
type Protocol string

const (
	ProtocolTestlib Protocol = "testlib"
	// ICPC programs accept with exit 42 and reject with 43; checkers and interactors also get a feedback directory.
	ProtocolICPC Protocol = "icpc"
)

const (
	icpcAccept   = 42
	icpcReject   = 43
	feedbackDir  = "feedback/"
	judgeMessage = feedbackDir + "judgemessage.txt"
)

// Jury is a compiled checker, interactor or validator; Args follow the arguments of its protocol.
type Jury struct {
	Executable Executable
	Language   string
	Protocol   Protocol
	Args       []string
}

// With appends arguments, such as those of a test group, after the jury's own.
func (j Jury) With(args []string) Jury {
	j.Args = append(slices.Clone(j.Args), args...)
	return j
}

// icpcFeedback lets an ICPC checker or interactor write the feedback directory it gets after input and answer.
func icpcFeedback(c *cmd) {
	c.CopyIn[feedbackDir+".keep"] = *memoryFile("")
	c.CopyOut = append(c.CopyOut, judgeMessage+"?")
}

// verdict reads a checker or interactor result; a program that did not exit on its own failed.
func (j Jury) verdict(r result, stdout string) (Status, *float64, string) {
	message := r.Files["stderr"]
	if r.Status != resultAccepted && r.Status != resultNonzeroExit {
		return StatusFail, nil, message
	}
	if j.Protocol != ProtocolICPC {
		status, score := parseTestlibVerdict(r.ExitStatus, stdout, message)
		return status, score, message
	}
	if m := r.Files[judgeMessage]; m != "" {
		message = m
	}
	switch r.ExitStatus {
	case icpcAccept:
		return StatusOK, nil, message
	case icpcReject:
		return StatusWA, nil, message
	default:
		return StatusFail, nil, message
	}
}

type ValidatorResult struct {
	// OK accepts and WA rejects the input; other statuses mean the validator gave no answer.
	Status  Status
	Message string
}

// ValidateFiles passes a testlib validator --group when group is set.
func (s *Sandbox) ValidateFiles(ctx context.Context, v Jury, input File, group string, limits FileRunLimits) (*ValidatorResult, error) {
	limits.Output = limits.Diagnostic
	var args []string
	if group != "" && v.Protocol != ProtocolICPC {
		args = []string{"--group", group}
	}
	c, err := s.fileCommand(v.Executable, v.Language, input, append(args, v.Args...), nil, limits)
	if err != nil {
		return nil, err
	}
	c.CopyOutCached = nil
	results, err := s.client.run(ctx, request{Cmd: []cmd{*c}})
	if err != nil {
		return nil, fmt.Errorf("validator file execution: %w", err)
	}
	r := results[0]
	result := &ValidatorResult{Status: statusToString(r.Status), Message: r.Files["stderr"]}
	exited := r.Status == resultAccepted || r.Status == resultNonzeroExit
	switch {
	case !exited && r.Status != resultSignalled:
	case v.Protocol != ProtocolICPC && r.ExitStatus == 0 && r.Status == resultAccepted:
		result.Status = StatusOK
	case v.Protocol != ProtocolICPC:
		result.Status = StatusWA
	case exited && r.ExitStatus == icpcAccept:
		result.Status = StatusOK
	case exited && r.ExitStatus == icpcReject:
		result.Status = StatusWA
	default:
		result.Status = StatusFail
	}
	return result, nil
}
