package prd

import (
	"strings"
	"unicode"
)

type catalogEntry struct {
	Key      string
	Required bool
}

// Canonical section keys. Matching is case-insensitive after stripping numbers and punctuation.
var sectionCatalog = map[string]catalogEntry{
	"product overview":            {Key: "product_overview", Required: true},
	"product definition":          {Key: "product_overview", Required: true},
	"overview":                    {Key: "product_overview", Required: true},
	"problem":                     {Key: "problem", Required: false},
	"product vision":              {Key: "product_vision", Required: false},
	"vision":                      {Key: "product_vision", Required: false},
	"goals":                       {Key: "goals", Required: true},
	"primary goals":               {Key: "goals", Required: true},
	"non goals":                   {Key: "non_goals", Required: false},
	"non goals for v1":            {Key: "non_goals", Required: false},
	"initial environment":         {Key: "initial_environment", Required: false},
	"design principles":           {Key: "design_principles", Required: false},
	"core design principles":      {Key: "design_principles", Required: false},
	"high level architecture":     {Key: "architecture", Required: false},
	"architecture":                {Key: "architecture", Required: false},
	"project state machine":       {Key: "project_state_machine", Required: false},
	"project state":               {Key: "project_state", Required: false},
	"project directory":           {Key: "project_directory", Required: false},
	"prd contract":                {Key: "prd_contract", Required: false},
	"requirement traceability":    {Key: "requirement_traceability", Required: false},
	"requirements":                {Key: "requirements", Required: false},
	"acceptance criteria":         {Key: "acceptance_criteria", Required: false},
	"preflight":                   {Key: "preflight", Required: false},
	"dependency handling":         {Key: "dependency_handling", Required: false},
	"dependencies":                {Key: "dependency_handling", Required: false},
	"credential handling":         {Key: "credential_handling", Required: false},
	"credentials":                 {Key: "credential_handling", Required: false},
	"clarification engine":        {Key: "clarification_engine", Required: false},
	"human intervention forecast": {Key: "human_intervention_forecast", Required: false},
	"human notification":          {Key: "human_notification", Required: false},
	"human validation":            {Key: "human_validation", Required: false},
	"phase graph":                 {Key: "phase_graph", Required: false},
	"phase review":                {Key: "phase_review", Required: false},
	"phases":                      {Key: "phases", Required: false},
	"initial development phases":  {Key: "phases", Required: false},
	"design planning":             {Key: "design_planning", Required: false},
	"design":                      {Key: "design_planning", Required: false},
	"design review":               {Key: "design_review", Required: false},
	"model router":                {Key: "model_router", Required: false},
	"subagent router":             {Key: "subagent_router", Required: false},
	"implementation worker":       {Key: "implementation_worker", Required: false},
	"autonomous permissions":      {Key: "autonomous_permissions", Required: false},
	"testing engine":              {Key: "testing_engine", Required: false},
	"testing":                     {Key: "testing_engine", Required: false},
	"adversarial testing":         {Key: "adversarial_testing", Required: false},
	"regression testing":          {Key: "regression_testing", Required: false},
	"review engine":               {Key: "review_engine", Required: false},
	"quality gate":                {Key: "quality_gate", Required: false},
	"failure diagnosis":           {Key: "failure_diagnosis", Required: false},
	"self fixing loop":            {Key: "self_fixing_loop", Required: false},
	"three attempt rule":          {Key: "three_attempt_rule", Required: false},
	"human debugging report":      {Key: "human_debugging_report", Required: false},
	"checkpoints":                 {Key: "checkpoints", Required: false},
	"rewind and replay":           {Key: "rewind_and_replay", Required: false},
	"learning engine":             {Key: "learning_engine", Required: false},
	"knowledge base":              {Key: "knowledge_base", Required: false},
	"knowledge promotion":         {Key: "knowledge_promotion", Required: false},
	"cost tracking":               {Key: "cost_tracking", Required: false},
	"human time tracking":         {Key: "human_time_tracking", Required: false},
	"project completion":          {Key: "project_completion", Required: false},
	"cli":                         {Key: "cli", Required: false},
	"status output":               {Key: "status_output", Required: false},
	"github integration":          {Key: "github_integration", Required: false},
	"ci":                          {Key: "ci", Required: false},
	"mcp":                         {Key: "mcp", Required: false},
	"infrastructure":              {Key: "infrastructure", Required: false},
	"benchmark projects":          {Key: "benchmark_projects", Required: false},
	"documentation":               {Key: "documentation", Required: false},
	"v1 success criteria":         {Key: "v1_success_criteria", Required: false},
	"ultimate success criteria":   {Key: "ultimate_success_criteria", Required: false},
	"final principle":             {Key: "final_principle", Required: false},
	"technical stack":             {Key: "technical_stack", Required: false},
	"security":                    {Key: "security", Required: false},
}

var requiredKeys = []string{"product_overview", "goals"}

var recommendedOptional = []string{"non_goals", "requirements", "phases"}

func normalizeHeading(title string) string {
	title = strings.TrimSpace(title)
	title = strings.TrimLeft(title, "#")
	title = strings.TrimSpace(title)
	title = strings.TrimRight(title, "#")
	title = strings.TrimSpace(title)
	runes := []rune(title)
	i := 0
	for i < len(runes) && unicode.IsDigit(runes[i]) {
		i++
	}
	if i > 0 && i < len(runes) && runes[i] == '.' {
		rest := strings.TrimSpace(string(runes[i+1:]))
		if rest != "" {
			title = rest
		}
	}
	title = strings.ToLower(title)
	repl := strings.NewReplacer("→", " ", "—", " ", "–", " ", "-", " ", "/", " ", ":", " ", ",", " ")
	title = repl.Replace(title)
	return strings.Join(strings.Fields(title), " ")
}

func lookupSection(title string) (catalogEntry, bool) {
	key := normalizeHeading(title)
	ent, ok := sectionCatalog[key]
	return ent, ok
}

func summary(s string, n int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n]) + "…"
}
