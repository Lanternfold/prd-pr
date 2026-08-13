package prd

import "fmt"

func validate(doc *Document) {
	seenReq := map[RequirementID]int{}
	for _, r := range doc.Requirements {
		if n, ok := seenReq[r.ID]; ok {
			doc.Diagnostics = append(doc.Diagnostics, Diagnostic{
				Severity:  SevError,
				Code:      "REQ_DUPLICATE",
				Message:   fmt.Sprintf("duplicate requirement ID %s (first seen at line %d)", r.ID, n),
				File:      r.Source.File,
				Section:   r.Source.Section,
				StartLine: r.Source.StartLine,
				EndLine:   r.Source.EndLine,
			})
		} else {
			seenReq[r.ID] = r.Source.StartLine
		}
		if r.Text == "" && r.Title == "" {
			doc.Diagnostics = append(doc.Diagnostics, Diagnostic{
				Severity:  SevError,
				Code:      "REQ_MALFORMED",
				Message:   fmt.Sprintf("requirement %s is missing a description", r.ID),
				File:      r.Source.File,
				Section:   r.Source.Section,
				StartLine: r.Source.StartLine,
				EndLine:   r.Source.EndLine,
			})
		}
	}

	seenAC := map[AcceptanceID]int{}
	for _, a := range doc.Acceptance {
		if n, ok := seenAC[a.ID]; ok {
			doc.Diagnostics = append(doc.Diagnostics, Diagnostic{
				Severity:  SevError,
				Code:      "AC_DUPLICATE",
				Message:   fmt.Sprintf("duplicate acceptance criteria ID %s (first seen at line %d)", a.ID, n),
				File:      a.Source.File,
				Section:   a.Source.Section,
				StartLine: a.Source.StartLine,
				EndLine:   a.Source.EndLine,
			})
		} else {
			seenAC[a.ID] = a.Source.StartLine
		}
		if a.Text == "" && a.Title == "" {
			doc.Diagnostics = append(doc.Diagnostics, Diagnostic{
				Severity:  SevError,
				Code:      "MISSING_REQUIRED_FIELD",
				Message:   fmt.Sprintf("acceptance criteria %s is missing a description", a.ID),
				File:      a.Source.File,
				Section:   a.Source.Section,
				StartLine: a.Source.StartLine,
				EndLine:   a.Source.EndLine,
			})
		}
	}

	seenTest := map[TestID]int{}
	for _, t := range doc.Tests {
		if n, ok := seenTest[t.ID]; ok {
			doc.Diagnostics = append(doc.Diagnostics, Diagnostic{
				Severity:  SevError,
				Code:      "TEST_DUPLICATE",
				Message:   fmt.Sprintf("duplicate test ID %s (first seen at line %d)", t.ID, n),
				File:      t.Source.File,
				Section:   t.Source.Section,
				StartLine: t.Source.StartLine,
				EndLine:   t.Source.EndLine,
			})
		} else {
			seenTest[t.ID] = t.Source.StartLine
		}
	}

	seenPhase := map[PhaseID]int{}
	for _, p := range doc.Phases {
		if n, ok := seenPhase[p.ID]; ok {
			doc.Diagnostics = append(doc.Diagnostics, Diagnostic{
				Severity:  SevError,
				Code:      "PHASE_DUPLICATE",
				Message:   fmt.Sprintf("duplicate phase ID %s (first seen at line %d)", p.ID, n),
				File:      p.Source.File,
				Section:   p.Source.Section,
				StartLine: p.Source.StartLine,
				EndLine:   p.Source.EndLine,
			})
		} else {
			seenPhase[p.ID] = p.Source.StartLine
		}
		if p.Name == "" && p.Objective == "" && p.ID != "" {
			// PHASE_MALFORMED already emitted for bare headers; still catch empty name.
			hasMalformed := false
			for _, d := range doc.Diagnostics {
				if d.Code == "PHASE_MALFORMED" && d.StartLine == p.Source.StartLine {
					hasMalformed = true
					break
				}
			}
			if !hasMalformed {
				doc.Diagnostics = append(doc.Diagnostics, Diagnostic{
					Severity:  SevError,
					Code:      "PHASE_MALFORMED",
					Message:   fmt.Sprintf("phase %s is missing a name", p.ID),
					File:      p.Source.File,
					Section:   p.Source.Section,
					StartLine: p.Source.StartLine,
					EndLine:   p.Source.EndLine,
				})
			}
		}
		for _, rid := range p.Requirements {
			if _, ok := seenReq[rid]; !ok {
				doc.Diagnostics = append(doc.Diagnostics, Diagnostic{
					Severity:  SevWarning,
					Code:      "UNKNOWN_REQ_REF",
					Message:   fmt.Sprintf("phase %s references unknown requirement %s", p.ID, rid),
					File:      p.Source.File,
					Section:   p.Source.Section,
					StartLine: p.Source.StartLine,
					EndLine:   p.Source.EndLine,
				})
			}
		}
	}

	present := map[string]bool{}
	for _, sec := range doc.Sections {
		present[sec.Key] = true
		if sec.Kind == KindUnknown {
			doc.Diagnostics = append(doc.Diagnostics, Diagnostic{
				Severity:  SevWarning,
				Code:      "UNKNOWN_SECTION",
				Message:   fmt.Sprintf("unknown section %q preserved", sec.Title),
				File:      sec.Source.File,
				Section:   sec.Title,
				StartLine: sec.Source.StartLine,
				EndLine:   sec.Source.EndLine,
			})
		}
	}
	for _, key := range requiredKeys {
		if !present[key] {
			doc.Diagnostics = append(doc.Diagnostics, Diagnostic{
				Severity: SevWarning,
				Code:     "MISSING_REQUIRED_SECTION",
				Message:  fmt.Sprintf("required section %s is absent", key),
				File:     doc.SourceFile,
			})
		}
	}
	for _, key := range recommendedOptional {
		if !present[key] {
			doc.Diagnostics = append(doc.Diagnostics, Diagnostic{
				Severity: SevInfo,
				Code:     "MISSING_OPTIONAL_SECTION",
				Message:  fmt.Sprintf("optional section %s is absent", key),
				File:     doc.SourceFile,
			})
		}
	}
}
