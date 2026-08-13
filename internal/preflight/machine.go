package preflight

import "strings"

const (
	binCursorAgent = "cursor-agent"
	binAgent       = "agent"
	binCursor      = "cursor"
	binGH          = "gh"
	binGit         = "git"
	binGo          = "go"
)

// Machine is cached host-tool observation. It does not inspect a product tree.
type Machine struct {
	GOOS            string
	GOARCH          string
	GoVersion       string
	GitPath         string
	GitAvailable    bool
	GoPath          string
	GoAvailable     bool
	CursorEditor    bool
	CursorAgent     bool
	CursorAgentBin  string
	CursorEditorBin string
	GitHubCLI       bool
	GitHubCLIPath   string
	AgentFromEnv    bool
}

// InspectMachine records PATH presence only. It does not invoke tools.
func InspectMachine(env Env) Machine {
	m := Machine{
		GOOS:      env.GOOS,
		GOARCH:    env.GOARCH,
		GoVersion: env.GoVersion,
	}
	if p, ok := env.hasBinary(binGit); ok {
		m.GitAvailable = true
		m.GitPath = p
	}
	if p, ok := env.hasBinary(binGo); ok {
		m.GoAvailable = true
		m.GoPath = p
	}
	if p, ok := env.hasBinary(binCursor); ok {
		m.CursorEditor = true
		m.CursorEditorBin = p
	}
	if envName, ok := env.lookupEnv()("CURSOR_AGENT_BIN"); ok && strings.TrimSpace(envName) != "" {
		m.CursorAgent = true
		m.CursorAgentBin = "CURSOR_AGENT_BIN"
		m.AgentFromEnv = true
	} else {
		for _, name := range []string{binCursorAgent, binAgent} {
			if p, ok := env.hasBinary(name); ok {
				m.CursorAgent = true
				m.CursorAgentBin = p
				break
			}
		}
	}
	if p, ok := env.hasBinary(binGH); ok {
		m.GitHubCLI = true
		m.GitHubCLIPath = p
	}
	return m
}

func addMachineChecks(r *Report, m Machine) {
	gitStatus := StatusMissing
	gitBlock := true
	gitDetail := "Git is not available on PATH"
	if m.GitAvailable {
		gitStatus = StatusAvailable
		gitBlock = false
		gitDetail = "available"
	}
	r.add(Check{ID: "machine.git", Name: "Git", Scope: ScopeMachine, Status: gitStatus, Blocking: gitBlock, Detail: gitDetail})

	edStatus := StatusMissing
	edDetail := "editor CLI not found"
	if m.CursorEditor {
		edStatus = StatusAvailable
		edDetail = "available (not the Agent CLI)"
	}
	r.add(Check{ID: "machine.cursor_editor", Name: "Cursor Editor", Scope: ScopeMachine, Status: edStatus, Blocking: false, Detail: edDetail})

	agStatus := StatusMissing
	agBlock := true
	agDetail := "Cursor Agent CLI not found (cursor-agent or agent)"
	if m.CursorAgent {
		agStatus = StatusAvailable
		agBlock = false
		agDetail = "available"
		if m.AgentFromEnv {
			agDetail = "available (CURSOR_AGENT_BIN is set)"
		}
	} else if m.CursorEditor {
		agDetail = "missing; editor cursor is not the Agent CLI"
	}
	r.add(Check{ID: "machine.cursor_agent", Name: "Cursor Agent", Scope: ScopeMachine, Status: blockingStatus(agBlock, agStatus), Blocking: agBlock, Detail: agDetail})

	ghStatus := StatusOptional
	ghDetail := "not required for current phase"
	if m.GitHubCLI {
		ghStatus = StatusAvailable
		ghDetail = "CLI available; authentication not required for current phase"
	} else {
		ghStatus = StatusOptional
		ghDetail = "missing; optional until GitHub integration"
	}
	r.add(Check{ID: "machine.github_cli", Name: "GitHub CLI", Scope: ScopeMachine, Status: ghStatus, Blocking: false, Detail: ghDetail})

	goStatus := StatusOptional
	goDetail := "not declared as required"
	if m.GoAvailable {
		goStatus = StatusAvailable
		goDetail = "available"
	} else {
		goStatus = StatusOptional
		goDetail = "missing; not required unless the project declares Go"
	}
	r.add(Check{ID: "machine.go", Name: "Go", Scope: ScopeMachine, Status: goStatus, Blocking: false, Detail: goDetail})
}

func blockingStatus(block bool, present Status) Status {
	if block {
		return StatusBlocking
	}
	return present
}
