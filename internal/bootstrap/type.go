package bootstrap

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/lanternfold/prd-pr/internal/prd"
)

const (
	TypeGoLibrary = "go_library"
	TypeGoCLI     = "go_cli"
	TypeIOS       = "ios"
	TypeAndroid   = "android"
	TypeWeb       = "web"
	TypeDesktop   = "desktop"
	TypeWriting   = "writing"
	TypeDesign    = "design"
	TypeData      = "data"
	TypeUnknown   = "unknown"
)

var (
	reAmbiguous = regexp.MustCompile(`(?i)\b((ios|iphone)\s+or\s+android|android\s+or\s+ios|web\s+or\s+native|native\s+or\s+web|cross-platform( mobile)? app)\b`)
	reIOS       = regexp.MustCompile(`(?i)\b(ios|iphone|ipad|swiftui|xcode)\b`)
	reAndroid   = regexp.MustCompile(`(?i)\b(android|kotlin|jetpack compose)\b`)
	reWeb       = regexp.MustCompile(`(?i)\b(web app|browser|react|next\.js|frontend web)\b`)
	reDesktop   = regexp.MustCompile(`(?i)\b(macos app|windows app|desktop app|electron)\b`)
	reCLI       = regexp.MustCompile(`(?i)\b(cli|command[ -]?line)\b`)
	reLibrary   = regexp.MustCompile(`(?i)\b(library|go module|package)\b`)
	reGo        = regexp.MustCompile(`(?i)\b(golang|\bgo\b|go\.mod)\b`)
	reWriting   = regexp.MustCompile(`(?i)\b(essay|article|book chapter)\b`)
	reDesign    = regexp.MustCompile(`(?i)\b(figma file|design system only)\b`)
)

// Selection is a deterministic project-type and Studio category decision.
type Selection struct {
	Type      string
	Category  string
	Slug      string
	Reason    string
	Ambiguous bool
	Question  string
}

// SelectType uses explicit PRD platform/type and Studio conventions. It does not invent a product type from vague wording.
func SelectType(doc *prd.Document) Selection {
	sel := Selection{Type: TypeUnknown, Category: "Products"}
	if doc == nil {
		sel.Ambiguous = true
		sel.Question = "The PRD could not be parsed; specify the product type and Studio category."
		return sel
	}
	sel.Slug = slug(firstNonEmpty(doc.Metadata.Product, doc.Metadata.Title, "product"))
	lower := strings.ToLower(doc.Metadata.Product + " " + doc.Metadata.Title)
	for _, s := range doc.Sections {
		lower += " " + strings.ToLower(s.Title+" "+s.Body)
	}
	lower += " " + strings.ToLower(strings.Join(doc.Goals, " "))

	if reAmbiguous.MatchString(lower) {
		sel.Ambiguous = true
		sel.Question = "The PRD names more than one runtime platform. Specify a single target (iOS, Android, web, desktop, CLI, or library)."
		sel.Reason = "ambiguous platform"
		return sel
	}

	hits := 0
	mark := func(t, cat, reason string) {
		hits++
		sel.Type, sel.Category, sel.Reason = t, cat, reason
	}
	if reIOS.MatchString(lower) && !reAndroid.MatchString(lower) {
		mark(TypeIOS, "Products", "explicit iOS platform")
	}
	if reAndroid.MatchString(lower) && !reIOS.MatchString(lower) {
		mark(TypeAndroid, "Products", "explicit Android platform")
	}
	if reWeb.MatchString(lower) {
		mark(TypeWeb, "Products", "explicit web platform")
	}
	if reDesktop.MatchString(lower) {
		mark(TypeDesktop, "Products", "explicit desktop platform")
	}
	if reWriting.MatchString(lower) {
		mark(TypeWriting, "Writing", "explicit writing product")
	}
	if reDesign.MatchString(lower) {
		mark(TypeDesign, "Design", "explicit design product")
	}

	if hits > 1 {
		sel.Ambiguous = true
		sel.Type = TypeUnknown
		sel.Question = "The PRD matches more than one product type. State the intended Studio category and runtime."
		sel.Reason = "conflicting explicit types"
		return sel
	}
	if hits == 1 {
		return sel
	}

	goish := reGo.MatchString(lower) || reLibrary.MatchString(lower) || reCLI.MatchString(lower)
	if goish {
		if reCLI.MatchString(lower) && !reLibrary.MatchString(lower) {
			sel.Type = TypeGoCLI
			sel.Category = "Tools"
			sel.Reason = "Go CLI convention"
			return sel
		}
		sel.Type = TypeGoLibrary
		sel.Category = "Tools"
		sel.Reason = "Go library/CLI Studio default"
		return sel
	}

	sel.Ambiguous = true
	sel.Question = "Project type and Studio location cannot be determined safely. Specify library/CLI (Tools), user-facing product (Products), or another Studio category."
	sel.Reason = "no explicit platform"
	return sel
}

func slug(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	prevHyphen := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevHyphen = false
			continue
		}
		if !prevHyphen && b.Len() > 0 {
			b.WriteByte('-')
			prevHyphen = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "product"
	}
	return out
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func Destination(studioRoot, category, slug string) string {
	return filepath.Join(studioRoot, category, slug)
}
