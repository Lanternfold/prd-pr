package prd

import (
	"os"
	"regexp"
	"strings"
)

// ParseFile reads UTF-8 Markdown from path. The file is treated as data only.
func ParseFile(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(path, string(data)), nil
}

// Parse converts Markdown into a Document. It never calls a network or an LLM.
//
// Approach: line-oriented split of ATX headings and catalog-matched numbered
// outline titles (e.g. "4. Goals"). Fenced code is not interpreted. goldmark
// is not used because this format includes outline lines that are not headings,
// and P2 forbids extra dependencies.
func Parse(sourceFile, markdown string) *Document {
	markdown = strings.TrimPrefix(markdown, "\uFEFF")
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	markdown = strings.ReplaceAll(markdown, "\r", "\n")

	doc := &Document{
		SchemaVersion: SchemaVersion,
		SourceFile:    sourceFile,
	}
	if strings.TrimSpace(markdown) == "" {
		doc.Diagnostics = append(doc.Diagnostics, Diagnostic{
			Severity: SevError,
			Code:     "EMPTY_PRD",
			Message:  "PRD is empty",
			File:     sourceFile,
		})
		return doc
	}

	blocks, splitDiags := splitSections(sourceFile, markdown)
	doc.Diagnostics = append(doc.Diagnostics, splitDiags...)
	extractMetadata(doc, blocks)
	for _, b := range blocks {
		if b.kind == KindPreamble && strings.TrimSpace(b.body) == "" && b.title == "" {
			continue
		}
		if b.kind == KindPreamble && !b.isTitle {
			continue
		}
		sec := Section{
			Key:    b.key,
			Title:  b.title,
			Kind:   b.kind,
			Source: b.src,
			Body:   b.body,
		}
		if b.isTitle && b.kind == KindPreamble {
			continue
		}
		doc.Sections = append(doc.Sections, sec)
	}

	extractFromSections(doc)
	validate(doc)
	return doc
}

type rawSection struct {
	title   string
	key     string
	kind    SectionKind
	src     SourceRef
	body    string
	heading string
	isTitle bool
	lines   []string
}

var (
	reATX         = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
	reNumbered    = regexp.MustCompile(`^(\d+)\.\s+(\S.*)$`)
	reMetaField   = regexp.MustCompile(`(?i)^\*\*(status|product|owner|repository):\*\*\s*(.+?)\s*$`)
	reItemID      = regexp.MustCompile(`^(?:#{1,6}\s+|[-*+]\s+)?((?:REQ|AC|TEST)-[A-Za-z0-9_-]+)(?:\s*[:.]\s+|\s+[—–-]\s+|\s+)?(.*)$`)
	rePhaseHeader = regexp.MustCompile(`^(?:#{1,6}\s+)?(P[0-9]+[A-Z]?)\s*[:.—–-]+\s+(\S.*)$`)
	rePhaseBare   = regexp.MustCompile(`^(?:#{1,6}\s+)?(P[0-9]+[A-Z]?)\s*:\s*$`)
	reLabel       = regexp.MustCompile(`(?i)^(?:[-*+]\s+)?(id|name|objective|dependencies|inputs|outputs|requirements|acceptance criteria|acceptance|implementation tasks|tasks|build|tests|design work|risks|human validation|definition of done|dod)\s*:\s*(.*)$`)
	reListItem    = regexp.MustCompile(`^[-*+]\s+(.+)$`)
	reFence       = regexp.MustCompile(`^(` + "```" + `|~~~)`)
	reDepClass    = regexp.MustCompile(`(?i)^(AVAILABLE|MISSING|OPTIONAL|BLOCKING)\s*$`)
	reCredential  = regexp.MustCompile(`(?i)^(?:[-*+]\s+)?credential(?:s)?\s*:\s+(.+)$`)
)

func splitSections(file, markdown string) ([]rawSection, []Diagnostic) {
	lines := strings.Split(markdown, "\n")
	var out []rawSection
	var diags []Diagnostic
	cur := rawSection{
		title: "",
		key:   "preamble",
		kind:  KindPreamble,
		src:   SourceRef{File: file, StartLine: 1},
	}
	fenced := false
	sawTitle := false

	flush := func(end int) {
		cur.src.EndLine = end
		cur.src.Section = cur.title
		cur.body = strings.TrimRight(strings.Join(cur.lines, "\n"), "\n")
		out = append(out, cur)
	}

	sectionStart := func(line string) (start bool, title string, level int, isATX bool) {
		level, heading, isATX := parseATX(line)
		numberedTitle, isNumbered := parseNumberedOutline(line)
		if isATX {
			title = heading
			if level == 1 {
				return true, title, level, true
			}
			if level == 2 {
				if _, ok := lookupSection(heading); ok {
					return true, title, level, true
				}
			}
			return false, "", level, true
		}
		if isNumbered {
			if _, ok := lookupSection(numberedTitle); ok {
				return true, numberedTitle, 0, false
			}
		}
		return false, "", 0, false
	}

	for i, line := range lines {
		n := i + 1
		trim := strings.TrimSpace(line)
		start, title, level, isATX := sectionStart(line)
		if reFence.MatchString(trim) && !(start && !isATX) {
			fenced = !fenced
			cur.lines = append(cur.lines, line)
			continue
		}
		if fenced && !(start && !isATX) {
			cur.lines = append(cur.lines, line)
			continue
		}
		if fenced && start && !isATX {
			fenced = false
			diags = append(diags, Diagnostic{
				Severity:  SevWarning,
				Code:      "UNCLOSED_FENCE",
				Message:   "fenced code was unclosed; treating the next numbered section as a section boundary",
				File:      file,
				StartLine: n,
			})
		}

		if !start {
			cur.lines = append(cur.lines, line)
			continue
		}

		if cur.src.StartLine == 0 {
			cur.src.StartLine = n
		}
		flush(n - 1)
		ent, ok := lookupSection(title)
		kind := KindUnknown
		key := slugUnknown(title)
		if ok {
			kind = KindOptional
			if ent.Required {
				kind = KindRequired
			}
			key = ent.Key
		}
		isTitle := false
		if isATX && level == 1 && !sawTitle && !ok {
			isTitle = true
			kind = KindPreamble
			key = "title"
			sawTitle = true
		} else if isATX && level == 1 && !sawTitle {
			sawTitle = true
		}
		cur = rawSection{
			title:   strings.TrimSpace(title),
			key:     key,
			kind:    kind,
			src:     SourceRef{File: file, Section: strings.TrimSpace(title), StartLine: n},
			heading: line,
			isTitle: isTitle,
		}
	}
	flush(len(lines))
	if fenced {
		diags = append(diags, Diagnostic{
			Severity: SevWarning,
			Code:     "UNCLOSED_FENCE",
			Message:  "fenced code was unclosed at end of file",
			File:     file,
		})
	}
	return out, diags
}

func parseATX(line string) (level int, title string, ok bool) {
	m := reATX.FindStringSubmatch(line)
	if m == nil {
		return 0, "", false
	}
	return len(m[1]), strings.TrimSpace(strings.TrimRight(m[2], "#")), true
}

func parseNumberedOutline(line string) (title string, ok bool) {
	if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		return "", false
	}
	m := reNumbered.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	return strings.TrimSpace(m[1] + ". " + m[2]), true
}

func slugUnknown(title string) string {
	n := normalizeHeading(title)
	if n == "" {
		return "unknown"
	}
	return "unknown:" + strings.ReplaceAll(n, " ", "_")
}

func extractMetadata(doc *Document, blocks []rawSection) {
	for _, b := range blocks {
		if b.isTitle {
			doc.Metadata.Title = b.title
		}
		if doc.Metadata.Title == "" && strings.HasPrefix(strings.ToLower(b.title), "prd") {
			doc.Metadata.Title = b.title
		}
		scan := b.heading + "\n" + b.body
		if b.isTitle {
			scan = b.title + "\n" + b.body
		}
		for _, line := range strings.Split(scan, "\n") {
			if m := reMetaField.FindStringSubmatch(line); m != nil {
				val := strings.TrimSpace(m[2])
				switch strings.ToLower(m[1]) {
				case "status":
					doc.Metadata.Status = val
				case "product":
					doc.Metadata.Product = val
				case "owner":
					doc.Metadata.Owner = val
				case "repository":
					doc.Metadata.Repository = val
				}
			}
		}
	}
	if doc.Metadata.Title == "" && len(blocks) > 0 && blocks[0].title != "" {
		doc.Metadata.Title = blocks[0].title
	}
}

func extractFromSections(doc *Document) {
	for i := range doc.Sections {
		sec := &doc.Sections[i]
		switch sec.Key {
		case "goals":
			doc.Goals = append(doc.Goals, listItems(sec.Body)...)
		case "non_goals":
			doc.NonGoals = append(doc.NonGoals, listItems(sec.Body)...)
		case "design_planning", "design_review":
			doc.Design.Present = true
			if doc.Design.Summary == "" {
				doc.Design.Summary = summary(sec.Body, 240)
			}
		case "architecture", "technical_stack", "initial_environment":
			doc.Technical.Present = true
			if doc.Technical.Summary == "" {
				doc.Technical.Summary = summary(sec.Body, 240)
			}
		case "testing_engine", "adversarial_testing", "regression_testing":
			doc.Testing.Present = true
			if doc.Testing.Summary == "" {
				doc.Testing.Summary = summary(sec.Body, 240)
			}
		case "human_validation", "human_notification", "human_intervention_forecast", "clarification_engine":
			doc.Human.Present = true
			if doc.Human.Summary == "" {
				doc.Human.Summary = summary(sec.Body, 240)
			}
		case "dependency_handling":
			extractDependencies(doc, sec)
		case "credential_handling":
			extractCredentials(doc, sec)
		}
		extractIDItems(doc, sec)
		extractPhases(doc, sec)
	}
}

func listItems(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimRight(line, " ")
		if m := reListItem.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			text := strings.TrimSpace(m[1])
			if text != "" {
				out = append(out, text)
			}
		}
	}
	return out
}

func extractDependencies(doc *Document, sec *Section) {
	class := ""
	for i, line := range strings.Split(sec.Body, "\n") {
		trim := strings.TrimSpace(line)
		if reDepClass.MatchString(trim) {
			class = strings.ToUpper(trim)
			continue
		}
		if m := reListItem.FindStringSubmatch(trim); m != nil {
			doc.Dependencies = append(doc.Dependencies, Dependency{
				Name:  strings.TrimSpace(m[1]),
				Class: class,
				Source: SourceRef{
					File:      sec.Source.File,
					Section:   sec.Title,
					StartLine: sec.Source.StartLine + i + 1,
					EndLine:   sec.Source.StartLine + i + 1,
				},
			})
		}
	}
}

func extractCredentials(doc *Document, sec *Section) {
	for i, line := range strings.Split(sec.Body, "\n") {
		trim := strings.TrimSpace(line)
		if m := reCredential.FindStringSubmatch(trim); m != nil {
			name := strings.TrimSpace(m[1])
			if name == "" || looksSecretLike(name) {
				continue
			}
			doc.Credentials = append(doc.Credentials, CredentialInfo{
				Name: name,
				Source: SourceRef{
					File:      sec.Source.File,
					Section:   sec.Title,
					StartLine: sec.Source.StartLine + i + 1,
					EndLine:   sec.Source.StartLine + i + 1,
				},
			})
		}
	}
}

func looksSecretLike(s string) bool {
	t := strings.ToLower(s)
	if strings.Contains(t, "sk-") || strings.Contains(t, "ghp_") || strings.Contains(t, "token=") {
		return true
	}
	return false
}

func extractIDItems(doc *Document, sec *Section) {
	lines := strings.Split(sec.Body, "\n")
	dedicated := sec.Key == "requirements" || sec.Key == "acceptance_criteria" || sec.Key == "testing_engine"
	fenced := false
	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if reFence.MatchString(line) {
			fenced = !fenced
			i++
			continue
		}
		if fenced {
			i++
			continue
		}
		abs := sec.Source.StartLine + i + 1
		m := reItemID.FindStringSubmatch(line)
		if m == nil {
			i++
			continue
		}
		rawID, rest := m[1], strings.TrimSpace(m[2])
		prefix := strings.Split(rawID, "-")[0]
		if !dedicated && rest == "" {
			i++
			continue
		}
		start := abs
		bodyLines := []string{}
		if rest != "" {
			bodyLines = append(bodyLines, rest)
		}
		j := i + 1
		for j < len(lines) {
			ntrim := strings.TrimSpace(lines[j])
			if reFence.MatchString(ntrim) {
				break
			}
			if ntrim == "" {
				if len(bodyLines) == 0 {
					j++
					continue
				}
				break
			}
			if reItemID.MatchString(ntrim) || rePhaseHeader.MatchString(ntrim) || reATX.MatchString(lines[j]) {
				break
			}
			if dedicated && reListItem.MatchString(ntrim) && !reItemID.MatchString(ntrim) {
				bodyLines = append(bodyLines, strings.TrimSpace(reListItem.FindStringSubmatch(ntrim)[1]))
				j++
				continue
			}
			if rest == "" && dedicated {
				bodyLines = append(bodyLines, ntrim)
				j++
				continue
			}
			break
		}
		text := strings.TrimSpace(strings.Join(bodyLines, " "))
		end := start
		if j > i+1 {
			end = sec.Source.StartLine + j
		}
		src := SourceRef{File: sec.Source.File, Section: sec.Title, StartLine: start, EndLine: end}
		switch prefix {
		case "REQ":
			id, err := ParseRequirementID(rawID)
			if err != nil {
				doc.Diagnostics = append(doc.Diagnostics, diag(SevError, "INVALID_IDENTIFIER", err.Error(), src))
			} else {
				title, body := splitTitleBody(text)
				doc.Requirements = append(doc.Requirements, Requirement{ID: id, Title: title, Text: body, Source: src})
			}
		case "AC":
			id, err := ParseAcceptanceID(rawID)
			if err != nil {
				doc.Diagnostics = append(doc.Diagnostics, diag(SevError, "INVALID_IDENTIFIER", err.Error(), src))
			} else {
				title, body := splitTitleBody(text)
				doc.Acceptance = append(doc.Acceptance, Criterion{ID: id, Title: title, Text: body, Source: src})
			}
		case "TEST":
			id, err := ParseTestID(rawID)
			if err != nil {
				doc.Diagnostics = append(doc.Diagnostics, diag(SevError, "INVALID_IDENTIFIER", err.Error(), src))
			} else {
				title, body := splitTitleBody(text)
				doc.Tests = append(doc.Tests, TestItem{ID: id, Title: title, Text: body, Source: src})
			}
		}
		if j <= i {
			i++
		} else {
			i = j
		}
	}
}

func splitTitleBody(text string) (string, string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", ""
	}
	if i := strings.Index(text, ". "); i > 0 && i < 80 {
		return strings.TrimSpace(text[:i]), strings.TrimSpace(text[i+1:])
	}
	return text, text
}

func extractPhases(doc *Document, sec *Section) {
	lines := strings.Split(sec.Body, "\n")
	var cur *Phase
	flush := func() {
		if cur != nil {
			doc.Phases = append(doc.Phases, *cur)
			cur = nil
		}
	}
	fenced := false
	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		trim := strings.TrimSpace(raw)
		if reFence.MatchString(trim) {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		abs := sec.Source.StartLine + i + 1
		if m := rePhaseBare.FindStringSubmatch(trim); m != nil && !rePhaseHeader.MatchString(trim) {
			flush()
			src := SourceRef{File: sec.Source.File, Section: sec.Title, StartLine: abs, EndLine: abs}
			id, err := ParsePhaseID(m[1])
			if err != nil {
				doc.Diagnostics = append(doc.Diagnostics, diag(SevError, "INVALID_IDENTIFIER", err.Error(), src))
				continue
			}
			cur = &Phase{ID: id, Source: src}
			doc.Diagnostics = append(doc.Diagnostics, diag(SevError, "PHASE_MALFORMED", "phase "+string(id)+" is missing a name", src))
			continue
		}
		if m := rePhaseHeader.FindStringSubmatch(trim); m != nil {
			flush()
			src := SourceRef{File: sec.Source.File, Section: sec.Title, StartLine: abs, EndLine: abs}
			id, err := ParsePhaseID(m[1])
			if err != nil {
				doc.Diagnostics = append(doc.Diagnostics, diag(SevError, "INVALID_IDENTIFIER", err.Error(), src))
				continue
			}
			cur = &Phase{ID: id, Name: strings.TrimSpace(m[2]), Source: src}
			continue
		}
		if cur == nil {
			continue
		}
		cur.Source.EndLine = abs
		if m := reLabel.FindStringSubmatch(trim); m != nil {
			label := strings.ToLower(m[1])
			rest := strings.TrimSpace(m[2])
			vals := []string{}
			if rest != "" {
				vals = append(vals, rest)
			}
			j := i + 1
			for j < len(lines) {
				nt := strings.TrimSpace(lines[j])
				if nt == "" {
					j++
					continue
				}
				if rePhaseHeader.MatchString(nt) || rePhaseBare.MatchString(nt) || reLabel.MatchString(nt) {
					break
				}
				if li := reListItem.FindStringSubmatch(nt); li != nil {
					vals = append(vals, strings.TrimSpace(li[1]))
					j++
					continue
				}
				if rest == "" {
					vals = append(vals, nt)
					j++
					continue
				}
				break
			}
			applyPhaseField(cur, label, vals)
			if j > i+1 {
				i = j - 1
				cur.Source.EndLine = sec.Source.StartLine + i + 1
			}
		}
	}
	flush()
}

func applyPhaseField(p *Phase, label string, vals []string) {
	switch label {
	case "id":
		if len(vals) > 0 {
			if id, err := ParsePhaseID(vals[0]); err == nil {
				p.ID = id
			}
		}
	case "name":
		p.Name = strings.Join(vals, " ")
	case "objective":
		p.Objective = strings.Join(vals, " ")
	case "dependencies":
		p.Dependencies = append(p.Dependencies, parsePhaseIDs(vals)...)
	case "inputs":
		p.Inputs = append(p.Inputs, splitComma(vals)...)
	case "outputs":
		p.Outputs = append(p.Outputs, splitComma(vals)...)
	case "requirements":
		p.Requirements = append(p.Requirements, parseReqIDs(vals)...)
	case "acceptance criteria", "acceptance":
		p.AcceptanceCriteria = append(p.AcceptanceCriteria, parseACIDs(vals)...)
	case "implementation tasks", "tasks", "build":
		p.Tasks = append(p.Tasks, vals...)
	case "tests":
		p.Tests = append(p.Tests, parseTestIDs(vals)...)
	case "design work":
		p.DesignWork = append(p.DesignWork, vals...)
	case "risks":
		p.Risks = append(p.Risks, vals...)
	case "human validation":
		p.HumanValidation = strings.Join(vals, " ")
	case "definition of done", "dod":
		p.DefinitionOfDone = append(p.DefinitionOfDone, vals...)
	}
}

func splitComma(vals []string) []string {
	var out []string
	for _, v := range vals {
		for _, p := range strings.Split(v, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func parsePhaseIDs(vals []string) []PhaseID {
	var out []PhaseID
	for _, tok := range splitComma(vals) {
		tok = strings.TrimSpace(tok)
		if id, err := ParsePhaseID(tok); err == nil {
			out = append(out, id)
		}
	}
	return out
}

func parseReqIDs(vals []string) []RequirementID {
	var out []RequirementID
	for _, tok := range splitComma(vals) {
		if id, err := ParseRequirementID(tok); err == nil {
			out = append(out, id)
		} else if looksLikeIDToken(tok) || strings.HasPrefix(tok, "REQ-") {
			out = append(out, RequirementID(tok))
		}
	}
	return out
}

func parseACIDs(vals []string) []AcceptanceID {
	var out []AcceptanceID
	for _, tok := range splitComma(vals) {
		if id, err := ParseAcceptanceID(tok); err == nil {
			out = append(out, id)
		} else if strings.HasPrefix(tok, "AC-") {
			out = append(out, AcceptanceID(tok))
		}
	}
	return out
}

func parseTestIDs(vals []string) []TestID {
	var out []TestID
	for _, tok := range splitComma(vals) {
		if id, err := ParseTestID(tok); err == nil {
			out = append(out, id)
		} else if strings.HasPrefix(tok, "TEST-") {
			out = append(out, TestID(tok))
		}
	}
	return out
}

func diag(sev Severity, code, msg string, src SourceRef) Diagnostic {
	return Diagnostic{
		Severity:  sev,
		Code:      code,
		Message:   msg,
		File:      src.File,
		Section:   src.Section,
		StartLine: src.StartLine,
		EndLine:   src.EndLine,
	}
}
