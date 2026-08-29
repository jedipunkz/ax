package cmd

// Manual flag parsing for the agent subcommands.
//
// Those commands set DisableFlagParsing so arbitrary flags can be forwarded to
// the agent binary untouched; this file is the hand-rolled scanner that takes
// their place.

import (
	"fmt"
	"slices"
	"strings"
)

// The agent subcommands disable cobra flag parsing so that arbitrary flags
// can be forwarded to the agent binary. parseAgentFlags is the shared scanner
// behind the per-command wrappers below: it always extracts -n/--name, and
// optionally -a/-m/--agent and -f/--follow depending on spec. Everything else
// (including a literal "--" and all tokens after it) is left in rest.

// agentFlagSpec selects which optional flags parseAgentFlags recognises.
// Flags outside the spec pass through to rest untouched.
type agentFlagSpec struct {
	agentType bool // -a / -m / --agent / --agent=
	follow    bool // -f / --follow
	force     bool // -f / --force
}

// agentFlags holds the values extracted by parseAgentFlags.
type agentFlags struct {
	name      string
	agentType string
	follow    bool
	force     bool
	rest      []string
}

func parseAgentFlags(args []string, spec agentFlagSpec) (agentFlags, error) {
	var p agentFlags
	i := 0
	for i < len(args) {
		if args[i] == "--" {
			p.rest = append(p.rest, args[i:]...)
			break
		}
		switch {
		case (args[i] == "-n" || args[i] == "--name") && i+1 < len(args):
			p.name = args[i+1]
			i += 2
		case strings.HasPrefix(args[i], "--name="):
			p.name = strings.TrimPrefix(args[i], "--name=")
			i++
		case spec.agentType && (args[i] == "-a" || args[i] == "-m" || args[i] == "--agent") && i+1 < len(args):
			if err := validateAgentType(args[i+1]); err != nil {
				return agentFlags{}, err
			}
			p.agentType = args[i+1]
			i += 2
		case spec.agentType && strings.HasPrefix(args[i], "--agent="):
			candidate := strings.TrimPrefix(args[i], "--agent=")
			if err := validateAgentType(candidate); err != nil {
				return agentFlags{}, err
			}
			p.agentType = candidate
			i++
		case spec.follow && (args[i] == "-f" || args[i] == "--follow"):
			p.follow = true
			i++
		case spec.force && (args[i] == "-f" || args[i] == "--force"):
			p.force = true
			i++
		default:
			p.rest = append(p.rest, args[i])
			i++
		}
	}
	return p, nil
}

// validateAgentType rejects agent types containing path separators or spaces;
// the value is later executed as a bare binary name.
func validateAgentType(candidate string) error {
	if strings.ContainsAny(candidate, "/ \\") {
		return fmt.Errorf("invalid agent type %q: must be a plain binary name", candidate)
	}
	return nil
}

func errNameRequired() error {
	return fmt.Errorf("requires -n/--name to specify the agent ID or name")
}

// parseAgentTypeAndNameFlag extracts -a/-m/--agent and -n/--name from args.
// agentType is empty when neither flag is given; callers apply their own default.
func parseAgentTypeAndNameFlag(args []string) (agentType string, name string, rest []string, err error) {
	p, err := parseAgentFlags(args, agentFlagSpec{agentType: true})
	return p.agentType, p.name, p.rest, err
}

// parseNameFlag extracts -n/--name from args (before any -- separator).
// Unrecognised flags and positional arguments are returned in rest.
func parseNameFlag(args []string) (name string, rest []string) {
	p, _ := parseAgentFlags(args, agentFlagSpec{}) // no validated flags → cannot fail
	return p.name, p.rest
}

// parseNameFlagRequired is like parseNameFlag but returns an error if -n/--name is absent.
func parseNameFlagRequired(args []string) (name string, rest []string, err error) {
	name, rest = parseNameFlag(args)
	if name == "" {
		err = errNameRequired()
	}
	return
}

// parseNameAndForceFlags extracts -n/--name and -f/--force from args.
// The name is required.
func parseNameAndForceFlags(args []string) (name string, force bool, rest []string, err error) {
	p, _ := parseAgentFlags(args, agentFlagSpec{force: true})
	if p.name == "" {
		return "", false, nil, errNameRequired()
	}
	return p.name, p.force, p.rest, nil
}

// parseNameAndFollowFlags extracts -n/--name and -f/--follow from args. The
// name is required. Unlike the other wrappers, the "--" separator itself is
// dropped from rest (its tail is kept) so `agx agent logs` can reject any
// leftover arguments without tripping over the separator.
func parseNameAndFollowFlags(args []string) (name string, follow bool, rest []string, err error) {
	p, _ := parseAgentFlags(args, agentFlagSpec{follow: true})
	if p.name == "" {
		return "", false, nil, errNameRequired()
	}
	rest = p.rest
	if i := slices.Index(rest, "--"); i >= 0 {
		rest = append(rest[:i], rest[i+1:]...)
	}
	if len(rest) == 0 {
		rest = nil
	}
	return p.name, p.follow, rest, nil
}
