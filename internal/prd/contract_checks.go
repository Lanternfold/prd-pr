package prd

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var (
	reVagueReq           = regexp.MustCompile(`(?i)\b(delightful|premium feel|feel premium|intuitive|user-friendly|nice ux|as appropriate|as needed|tbd|todo|coming soon|somehow|or so)\b`)
	reDecideLater        = regexp.MustCompile(`(?i)\b(decide later|left to the implementer|up to the (agent|implementer|engineer)|implementation (may|can|should) (choose|decide)|to be determined)\b`)
	reGoShape            = regexp.MustCompile(`(?i)(\bgo\b|golang|\(.*int.*\)|\bfunc\b|go\.mod|module |\bcli\b|\blibrary\b|\bpackage\b)`)
	reChosenRuntime      = regexp.MustCompile(`(?i)\b(ios|iphone|ipad|android|web app|browser|desktop|macos|windows|linux|cli|command[ -]?line|library|go module|golang|\bgo\b)\b`)
	reAmbiguousPlatform  = regexp.MustCompile(`(?i)\b((ios|iphone)\s+or\s+android|android\s+or\s+ios|web\s+or\s+native|native\s+or\s+web|cross-platform( mobile)? app|mobile app)\b`)
	reNamedService       = regexp.MustCompile(`(?i)\b(stripe|openai|anthropic|aws|amazon s3|\bs3\b|postgres|postgresql|mysql|redis|oauth|twilio|slack|sendgrid|firebase|github api|google maps|stripe checkout)\b`)
	reCredNeed           = regexp.MustCompile(`(?i)\b(api key|oauth(?:2)?|access token|client secret|service account|password login)\b`)
	reSecuritySensitive  = regexp.MustCompile(`(?i)\b(log ?in|sign ?in|authenticat|password|oauth|encrypt|pii|personally identifiable|payment card|credit card|session token|jwt|authorization)\b`)
	reSecurityConstraint = regexp.MustCompile(`(?i)\b(hash|bcrypt|argon|tls|https only|at rest|in transit|no plaintext|keychain|sandbox|least privilege|do not store (the )?password)\b`)
	reTestableAC         = regexp.MustCompile(`(?i)\b(return|returns|equal|equals|must |shall |pass|fail|exit|status|output|given |when |then )\b`)
	reLocalRun           = regexp.MustCompile(`(?i)\b(go test|npm test|cargo test|how to run|local run|run locally)\b`)
)

func checkStructure(c *contractCtx) {
	if strings.TrimSpace(c.markdown) == "" {
		c.add(finding(ValStructure, FindBlocking, "STRUCTURE", "PRD",
			"PRD is empty.",
			"An empty document cannot define a product to implement.",
			"Write a PRD with product objective, requirements, acceptance criteria, and phases.",
			"", ""))
		return
	}
	for _, d := range c.doc.Diagnostics {
		if d.Severity != SevError {
			continue
		}
		c.add(finding(ValStructure, FindBlocking, "STRUCTURE", locOf(SourceRef{Section: d.Section, StartLine: d.StartLine}, d.Code),
			d.Message,
			"Parser errors mean the PRD is not a reliable implementation contract.",
			"Fix the structural error. Do not ask the implementation agent to guess the intended structure.",
			"", ""))
	}
	if !c.hasSection("product_overview") && !c.hasSection("goals") {
		c.add(finding(ValStructure, FindBlocking, "STRUCTURE", "PRD",
			"Required PRD structure is missing (product overview and goals).",
			"Without those sections the orchestrator cannot identify the product to build.",
			"Add Product Overview and Goals sections with concrete content.",
			"", ""))
	}
}

func checkObjective(c *contractCtx) {
	overview := strings.TrimSpace(c.sectionBody("product_overview"))
	goals := c.doc.Goals
	product := strings.TrimSpace(c.doc.Metadata.Product)
	if overview == "" && len(goals) == 0 && product == "" {
		c.add(finding(ValObjective, FindBlocking, "OBJECTIVE", "Product Overview",
			"Core product objective/problem is missing.",
			"The implementation agent cannot choose what to build without a stated problem and outcome.",
			"State the problem, who it is for, and the product objective. Ask the human if this is unknown; do not invent it.",
			"", ""))
		return
	}
	if overview == "" && len(goals) == 0 {
		c.add(finding(ValObjective, FindBlocking, "OBJECTIVE", "Product Overview",
			"Product overview and goals are empty.",
			"A title is not an implementation contract.",
			"Describe the problem and the objective in Product Overview or Goals.",
			"", ""))
	}
}

func checkScope(c *contractCtx) {
	hasNonGoals := len(c.doc.NonGoals) > 0 || nonempty(c.sectionBody("non_goals"))
	hasScope := c.hasSection("unknown:scope") || strings.Contains(c.lower, "in scope") || strings.Contains(c.lower, "out of scope") || hasNonGoals
	needsScope := len(c.doc.Requirements) >= 3 || reNamedService.MatchString(c.lower) || reSecuritySensitive.MatchString(c.lower)
	if hasScope {
		return
	}
	if needsScope {
		c.add(finding(ValScope, FindBlocking, "SCOPE", "Non-Goals",
			"Scope/non-scope is missing where it is necessary to determine implementation.",
			"Without non-scope, the agent would have to decide which adjacent features to build.",
			"State in-scope and out-of-scope behavior. Ask the human if the boundary is unknown.",
			"", ""))
		return
	}
	c.add(finding(ValScope, FindWarning, "SCOPE", "Non-Goals",
		"No explicit non-goals are declared.",
		"A small, single-function product can inherit a narrow Studio default, but larger products need an explicit boundary.",
		"Add Non-Goals if there is any adjacent behavior the agent might otherwise invent.",
		"", ""))
}

func checkRequirements(c *contractCtx) {
	if len(c.doc.Requirements) == 0 {
		c.add(finding(ValClarity, FindBlocking, "REQUIREMENT", "Requirements",
			"No functional requirements are declared.",
			"Autonomous implementation needs objectively stated requirements.",
			"Add REQ-* items that describe observable product behavior.",
			"", ""))
		return
	}
	for _, r := range c.doc.Requirements {
		text := strings.TrimSpace(r.Title + " " + r.Text)
		if text == "" {
			c.add(finding(ValClarity, FindBlocking, "REQUIREMENT", locOf(r.Source, string(r.ID)),
				fmt.Sprintf("Requirement %s has no description.", r.ID),
				"An empty requirement cannot be implemented or verified.",
				"Write an objective description, or remove the ID.",
				string(r.ID), ""))
			continue
		}
		if reVagueReq.MatchString(text) && !reTestableAC.MatchString(text) {
			c.add(finding(ValClarity, FindBlocking, "REQUIREMENT", locOf(r.Source, string(r.ID)),
				fmt.Sprintf("Requirement %s cannot be objectively understood.", r.ID),
				"Subjective language forces the agent to invent product taste and success criteria.",
				"Replace subjective wording with observable behavior. Ask the human for the missing decision.",
				string(r.ID), ""))
		}
	}
}

func checkAcceptance(c *contractCtx) {
	if len(c.doc.Requirements) == 0 {
		return
	}
	if len(c.doc.Acceptance) == 0 {
		c.add(finding(ValAcceptance, FindBlocking, "ACCEPTANCE", "Acceptance Criteria",
			"Requirements have no verifiable acceptance criteria.",
			"Autonomous verification needs pass/fail conditions the engine can evaluate or explicitly mark as manual.",
			"Add AC-* items that state observable outcomes for each requirement that must be verified.",
			"", ""))
		return
	}
	byNum := map[string]bool{}
	for _, a := range c.doc.Acceptance {
		byNum[strings.TrimPrefix(string(a.ID), "AC-")] = true
		text := strings.TrimSpace(a.Title + " " + a.Text)
		if text == "" {
			c.add(finding(ValAcceptance, FindBlocking, "ACCEPTANCE", locOf(a.Source, string(a.ID)),
				fmt.Sprintf("Acceptance criterion %s is empty.", a.ID),
				"Empty acceptance criteria cannot be verified.",
				"State a concrete pass/fail condition.",
				"", ""))
			continue
		}
		if reVagueReq.MatchString(text) && !reTestableAC.MatchString(text) {
			c.add(finding(ValAcceptance, FindBlocking, "ACCEPTANCE", locOf(a.Source, string(a.ID)),
				fmt.Sprintf("Acceptance criterion %s is not verifiable.", a.ID),
				"The engine cannot independently verify subjective success.",
				"Rewrite the criterion as an observable condition, or mark the verification as explicitly manual.",
				"", ""))
		}
	}
	for _, r := range c.doc.Requirements {
		num := strings.TrimPrefix(string(r.ID), "REQ-")
		covered := byNum[num]
		if !covered {
			for _, p := range c.doc.Phases {
				for _, id := range p.Requirements {
					if id == r.ID && len(p.AcceptanceCriteria) > 0 {
						covered = true
					}
				}
			}
		}
		if !covered {
			c.add(finding(ValAcceptance, FindBlocking, "ACCEPTANCE", locOf(r.Source, string(r.ID)),
				fmt.Sprintf("Requirement %s has no verifiable acceptance condition.", r.ID),
				"Unmapped requirements leave the agent to invent done.",
				"Add an AC-* (or phase acceptance mapping) for this requirement.",
				string(r.ID), ""))
		}
	}
}

func checkConflicts(c *contractCtx) {
	type pair struct{ a, b string }
	exclusives := []pair{
		{"only on the device", "only in the cloud"},
		{"only in the cloud", "only on the device"},
		{"must be public", "must be private"},
		{"http only", "https only"},
		{"must not persist", "must persist"},
		{"offline only", "online only"},
	}
	reqs := c.doc.Requirements
	for i := 0; i < len(reqs); i++ {
		ti := strings.ToLower(reqs[i].Title + " " + reqs[i].Text)
		for j := i + 1; j < len(reqs); j++ {
			tj := strings.ToLower(reqs[j].Title + " " + reqs[j].Text)
			for _, p := range exclusives {
				if strings.Contains(ti, p.a) && strings.Contains(tj, p.b) {
					c.add(finding(ValConflict, FindBlocking, "CONFLICT", locOf(reqs[i].Source, string(reqs[i].ID)),
						fmt.Sprintf("Requirements %s and %s conflict.", reqs[i].ID, reqs[j].ID),
						"Conflicting product rules force the agent to pick a winner the PRD did not authorize.",
						"Resolve the conflict in the PRD. Ask the human which rule is authoritative.",
						string(reqs[i].ID), ""))
					return
				}
			}
			if contradictsMust(ti, tj) {
				c.add(finding(ValConflict, FindBlocking, "CONFLICT", locOf(reqs[i].Source, string(reqs[i].ID)),
					fmt.Sprintf("Requirements %s and %s appear to contradict each other.", reqs[i].ID, reqs[j].ID),
					"The agent must not choose between contradictory mandates.",
					"Edit the PRD so only one behavior is required.",
					string(reqs[i].ID), ""))
				return
			}
		}
	}
}

func contradictsMust(a, b string) bool {
	mustNot := func(s, other string) bool {
		if !strings.Contains(s, "must not ") && !strings.Contains(s, "shall not ") {
			return false
		}
		for _, p := range []string{"must not ", "shall not "} {
			if i := strings.Index(s, p); i >= 0 {
				rest := strings.TrimSpace(s[i+len(p):])
				words := strings.FieldsFunc(rest, func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSpace(r) })
				if len(words) == 0 {
					return false
				}
				needle := strings.Join(words[:min(3, len(words))], " ")
				if needle != "" && (strings.Contains(other, "must "+needle) || strings.Contains(other, "shall "+needle)) {
					return true
				}
			}
		}
		return false
	}
	return mustNot(a, b) || mustNot(b, a)
}

func checkRuntime(c *contractCtx) {
	if reAmbiguousPlatform.MatchString(c.lower) && !singleChosenMobile(c.lower) {
		c.add(finding(ValRuntime, FindBlocking, "RUNTIME", locRuntime(c),
			"Target runtime platform is unspecified or ambiguous.",
			"The implementation strategy and verification mechanism depend on the runtime platform.",
			"Specify web, iOS, Android, desktop, CLI, library, or another single target. Ask the human if this is unknown.",
			"", ""))
		return
	}
	if reChosenRuntime.MatchString(c.lower) || reGoShape.MatchString(c.lower) {
		if !regexp.MustCompile(`(?i)\bgo 1\.\d+`).MatchString(c.lower) && reGoShape.MatchString(c.lower) {
			c.add(finding(ValRuntime, FindWarning, "RUNTIME", locRuntime(c),
				"Language/toolchain version is not specified.",
				"Studio has a safe default (current stable Go for library/CLI products). The omission does not change product behavior.",
				"Optionally pin a Go version if a specific version is required; otherwise the Studio default applies.",
				"", ""))
		}
		return
	}
	c.add(finding(ValRuntime, FindWarning, "RUNTIME", locRuntime(c),
		"Target runtime platform is not stated.",
		"For a headless library/CLI-shaped PRD, Studio defaults to a local Go module. That default does not change user-facing product behavior.",
		"State the runtime if the product is not a Go library/CLI. Do not leave UI platforms implicit.",
		"", ""))
}

func locRuntime(c *contractCtx) string {
	for _, key := range []string{"technical_stack", "initial_environment", "architecture", "product_overview"} {
		if s := c.section(key); s != nil {
			return locOf(s.Source, s.Title)
		}
	}
	return "PRD"
}

func singleChosenMobile(lower string) bool {
	ios := strings.Contains(lower, "ios") || strings.Contains(lower, "iphone")
	and := strings.Contains(lower, "android")
	return ios != and
}

func checkDependencies(c *contractCtx) {
	mentioned := uniqueMatches(reNamedService, c.lower)
	if len(mentioned) == 0 {
		return
	}
	declared := strings.ToLower(c.sectionBody("dependency_handling"))
	for _, name := range c.doc.Dependencies {
		declared += " " + strings.ToLower(name.Name)
	}
	var missing []string
	for _, m := range mentioned {
		if !strings.Contains(declared, m) {
			missing = append(missing, m)
		}
	}
	if len(missing) == 0 {
		return
	}
	c.add(finding(ValDependency, FindBlocking, "DEPENDENCY", "Dependencies",
		"Critical external dependencies are mentioned but not declared: "+strings.Join(missing, ", ")+".",
		"The agent cannot know what must exist before implementation, or what to treat as blocking vs optional.",
		"Declare each external dependency, what it is for, and whether it is required. Do not invent undeclared services.",
		"", ""))
}

func checkCredentials(c *contractCtx) {
	needs := reCredNeed.MatchString(c.lower) || namedServiceNeedsCred(c.lower)
	if !needs {
		return
	}
	if len(c.doc.Credentials) > 0 {
		for _, cred := range c.doc.Credentials {
			if looksSecretLike(cred.Name) {
				continue
			}
			if len(strings.Fields(cred.Name)) < 2 {
				c.add(finding(ValCredential, FindBlocking, "CREDENTIAL", locOf(cred.Source, "Credentials"),
					"Credential "+cred.Name+" is named without enough information to know what is required.",
					"Presence checks need a service and purpose, not a secret value.",
					"Declare the credential name, which service it authenticates, and what it is used for. Do not put secret values in the PRD.",
					"", ""))
			}
		}
		return
	}
	c.add(finding(ValCredential, FindBlocking, "CREDENTIAL", "Credentials",
		"The PRD requires credentials or secrets but does not declare what must be present.",
		"The orchestrator must ask for named credentials; it must not invent providers or store secret values.",
		"Add a Credentials section listing each required credential by name and purpose, with no secret values.",
		"", ""))
}

func namedServiceNeedsCred(lower string) bool {
	for _, s := range []string{"openai", "stripe", "twilio", "sendgrid", "aws", "anthropic"} {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

func checkPhases(c *contractCtx) {
	if len(c.doc.Phases) == 0 {
		c.add(finding(ValPhaseGraph, FindWarning, "PHASE_GRAPH", "Phases",
			"No implementation phases are declared.",
			"A single-phase product can proceed from requirements if definition of done is explicit; otherwise the agent would invent a work breakdown.",
			"Add at least one phase with objective, dependencies, and definition of done, or state that the work is a single delivery.",
			"", ""))
		return
	}
	seen := map[PhaseID]bool{}
	for _, p := range c.doc.Phases {
		seen[p.ID] = true
	}
	idx := map[PhaseID]int{}
	for i, p := range c.doc.Phases {
		idx[p.ID] = i
		for _, dep := range p.Dependencies {
			if dep == p.ID {
				c.add(finding(ValPhaseGraph, FindBlocking, "PHASE_GRAPH", locOf(p.Source, string(p.ID)),
					fmt.Sprintf("Phase %s depends on itself.", p.ID),
					"A self-dependency cannot be scheduled.",
					"Remove the self-dependency.",
					"", string(p.ID)))
			} else if !seen[dep] {
				c.add(finding(ValPhaseGraph, FindBlocking, "PHASE_GRAPH", locOf(p.Source, string(p.ID)),
					fmt.Sprintf("Phase %s depends on unknown phase %s.", p.ID, dep),
					"Impossible dependencies prevent a valid execution graph.",
					"Reference only declared phase IDs, or add the missing phase.",
					"", string(p.ID)))
			}
		}
	}
	if cycle := phaseCycle(c.doc.Phases); cycle != "" {
		c.add(finding(ValPhaseGraph, FindBlocking, "PHASE_GRAPH", "Phases",
			"Phase dependencies contain a cycle: "+cycle+".",
			"Cyclic phases cannot be ordered for autonomous implementation.",
			"Remove the cycle so every phase has a finite prerequisite chain.",
			"", ""))
	}
}

func phaseCycle(phases []Phase) string {
	deps := map[PhaseID][]PhaseID{}
	for _, p := range phases {
		deps[p.ID] = append([]PhaseID{}, p.Dependencies...)
	}
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[PhaseID]int{}
	var stack []PhaseID
	var found []PhaseID
	var dfs func(PhaseID) bool
	dfs = func(id PhaseID) bool {
		color[id] = gray
		stack = append(stack, id)
		for _, d := range deps[id] {
			if color[d] == gray {
				found = append(append([]PhaseID{}, stack...), d)
				return true
			}
			if color[d] == white && dfs(d) {
				return true
			}
		}
		stack = stack[:len(stack)-1]
		color[id] = black
		return false
	}
	for _, p := range phases {
		if color[p.ID] == white && dfs(p.ID) {
			parts := make([]string, 0, len(found))
			for _, id := range found {
				parts = append(parts, string(id))
			}
			return strings.Join(parts, " → ")
		}
	}
	return ""
}

func checkDoD(c *contractCtx) {
	hasDocDoD := c.hasSection("project_completion") || c.hasSection("v1_success_criteria") ||
		strings.Contains(c.lower, "definition of done")
	phaseDoD := 0
	for _, p := range c.doc.Phases {
		if len(p.DefinitionOfDone) > 0 {
			phaseDoD++
		}
	}
	if len(c.doc.Phases) > 0 && phaseDoD == 0 && !hasDocDoD {
		c.add(finding(ValDoD, FindBlocking, "DEFINITION_OF_DONE", "Phases",
			"Implementation outcome / definition of done is missing.",
			"Without a definition of done the agent cannot know when to stop.",
			"Add Definition of Done for each phase or for the product.",
			"", ""))
		return
	}
	if len(c.doc.Phases) == 0 && !hasDocDoD {
		c.add(finding(ValDoD, FindBlocking, "DEFINITION_OF_DONE", "PRD",
			"No definition of done is present.",
			"Autonomous delivery requires an explicit done condition.",
			"State what done means (tests, outputs, local run).",
			"", ""))
	}
}

func checkTesting(c *contractCtx) {
	if len(c.doc.Requirements) == 0 {
		return
	}
	hasTests := len(c.doc.Tests) > 0 || c.doc.Testing.Present || reLocalRun.MatchString(c.lower)
	if hasTests {
		return
	}
	c.add(finding(ValTesting, FindBlocking, "TESTING", "Testing",
		"Testing/verification expectations are missing.",
		"The engine must verify independently and cannot invent a test strategy that changes product meaning.",
		"Declare tests or an explicit verification method (commands, manual AC, or both).",
		"", ""))
}

func checkSecurity(c *contractCtx) {
	if !hasActiveSecuritySensitive(c.lower) {
		return
	}
	secBody := c.sectionBody("security") + " " + c.sectionBody("credential_handling")
	combined := strings.ToLower(secBody)
	if c.hasSection("security") && (reSecurityConstraint.MatchString(combined) ||
		regexp.MustCompile(`(?i)\bno (authentication|auth|login|pii|password|secret|network)\b`).MatchString(combined)) {
		return
	}
	if reSecurityConstraint.MatchString(c.lower) && nonempty(secBody) {
		return
	}
	c.add(finding(ValSecurity, FindBlocking, "SECURITY", locSecurity(c),
		"Security-sensitive behavior is specified without sufficient constraints.",
		"Auth, secrets, payments, and PII require explicit constraints; otherwise the agent would invent a security design.",
		"State required constraints (storage, transport, hashing, what must not be logged). Ask the human rather than inventing a threat model.",
		"", ""))
}

func locSecurity(c *contractCtx) string {
	if s := c.section("security"); s != nil {
		return locOf(s.Source, s.Title)
	}
	return "PRD"
}

func hasActiveSecuritySensitive(lower string) bool {
	idxs := reSecuritySensitive.FindAllStringIndex(lower, -1)
	if len(idxs) == 0 {
		return false
	}
	neg := regexp.MustCompile(`(?i)\b(no|not|without|none)\b`)
	for _, span := range idxs {
		start := span[0] - 24
		if start < 0 {
			start = 0
		}
		window := lower[start:span[0]]
		if neg.MatchString(window) {
			continue
		}
		return true
	}
	return false
}

func checkIntegrations(c *contractCtx) {
	mentioned := uniqueMatches(reNamedService, c.lower)
	if len(mentioned) == 0 {
		return
	}
	for _, name := range mentioned {
		if !integrationHasBehavior(c, name) {
			c.add(finding(ValIntegration, FindBlocking, "INTEGRATION", "External integrations",
				"External integration "+name+" is named but required behavior is undefined.",
				"The agent would have to invent events, payloads, failure handling, and success criteria.",
				"Specify what the product must send/receive, when, and how failures are handled. Ask the human if this is unknown.",
				"", ""))
		}
	}
}

func integrationHasBehavior(c *contractCtx, name string) bool {
	// Look in a window of text around the first mention.
	idx := strings.Index(c.lower, name)
	if idx < 0 {
		return false
	}
	start := idx - 80
	if start < 0 {
		start = 0
	}
	end := idx + 220
	if end > len(c.lower) {
		end = len(c.lower)
	}
	window := c.lower[start:end]
	return regexp.MustCompile(`(?i)\b(must|shall|when |on failure|retry|timeout|webhook|payload|event|channel|endpoint)\b`).MatchString(window)
}

func checkUnauthorizedDecisions(c *contractCtx) {
	if reDecideLater.MatchString(c.lower) {
		c.add(finding(ValDecision, FindBlocking, "DECISION", "PRD",
			"The PRD defers a material product decision to implementation.",
			"The implementation agent is not authorized to invent product decisions.",
			"Resolve the decision in the PRD, or ask the human. Do not leave it to the agent.",
			"", ""))
	}
	for _, r := range c.doc.Requirements {
		text := r.Title + " " + r.Text
		if reDecideLater.MatchString(text) {
			c.add(finding(ValDecision, FindBlocking, "DECISION", locOf(r.Source, string(r.ID)),
				fmt.Sprintf("Requirement %s requires a product decision the PRD does not make.", r.ID),
				"Guessing would change product behavior.",
				"Ask the human and record the authorized choice in the PRD.",
				string(r.ID), ""))
		}
	}
}

func uniqueMatches(re *regexp.Regexp, s string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range re.FindAllString(s, -1) {
		k := strings.ToLower(strings.TrimSpace(m))
		if k == "s3" {
			k = "s3"
		}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	sortStrings(out)
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
