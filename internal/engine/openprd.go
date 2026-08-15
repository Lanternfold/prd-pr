package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lanternfold/prd-pr/internal/apprun"
	"github.com/lanternfold/prd-pr/internal/bootstrap"
	"github.com/lanternfold/prd-pr/internal/human"
	"github.com/lanternfold/prd-pr/internal/prd"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/studio"
)

// OpenFromPRD is the PRD-only entry: validate, place the Studio project, then prepare.
// It does not invoke a Cursor worker.
func (e *Engine) OpenFromPRD(ctx context.Context, prdPath string) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	abs, err := filepath.Abs(prdPath)
	if err != nil {
		return Result{Execution: Execution{RefusalReason: err.Error()}}, nil
	}
	req := Request{PRDPath: abs, PRDOnly: true}
	if blocked, res := e.contractGate(req); blocked {
		return res, nil
	}
	doc, err := prd.ParseFile(abs)
	if err != nil {
		return Result{Execution: Execution{RefusalReason: "cannot read PRD: " + err.Error()}}, nil
	}
	if h, _ := e.reviewCompleteness(ctx, abs, doc); h.Kind != "" {
		res := Result{
			WaitingForHuman: true,
			Human:           &h,
			Execution:       Execution{RefusalReason: "product completeness review requires a human decision"},
		}
		return res, nil
	}

	sel := bootstrap.SelectType(doc)
	if sel.Ambiguous {
		h := human.Request{
			Kind:    human.KindStudioPlacement,
			Reason:  sel.Reason,
			Needed:  sel.Question,
			Urgency: human.UrgencyHigh,
		}
		return Result{WaitingForHuman: true, Human: &h, Execution: Execution{RefusalReason: sel.Question}}, nil
	}

	lay := e.studioLayout()
	if lay.Empty() {
		h := human.Request{
			Kind:    human.KindStudioPlacement,
			Reason:  "studio_undiscovered",
			Needed:  "Studio root could not be determined safely. Set PRDPR_STUDIO to the Studio directory that contains Tools/ and Products/, then retry.",
			Urgency: human.UrgencyHigh,
		}
		return Result{WaitingForHuman: true, Human: &h, Execution: Execution{RefusalReason: h.Needed}}, nil
	}
	if !lay.Has(sel.Category) {
		h := human.Request{
			Kind:    human.KindStudioPlacement,
			Reason:  "missing_category",
			Needed:  fmt.Sprintf("Studio category %s does not exist under %s. Create it or specify the correct category in the PRD.", sel.Category, lay.Root),
			Urgency: human.UrgencyHigh,
		}
		return Result{WaitingForHuman: true, Human: &h, Execution: Execution{RefusalReason: h.Needed}}, nil
	}

	dest := bootstrap.Destination(lay.Root, sel.Category, sel.Slug)
	if isOrchestratorRepo(dest) {
		if PRDDeclaresSelfDevelopment(abs) {
			return e.Prepare(ctx, Request{
				ProductRoot:   dest,
				PRDPath:       abs,
				PRDOnly:       true,
				ExecutionMode: state.ExecutionModeSelfDevelopment,
			})
		}
		if !e.opts.AllowSelf {
			return refused(dest, "refusing to use the PRD→PR orchestrator repository as a product workspace"), nil
		}
	}

	placed, err := bootstrap.Place(abs, dest, sel, doc)
	if err != nil {
		return Result{Execution: Execution{RefusalReason: err.Error()}}, nil
	}
	if placed.Conflict {
		h := human.Request{
			Kind:    human.KindDirectoryConflict,
			Reason:  "unsafe_adopt",
			Needed:  placed.Reason + " Choose another location or clean the directory.",
			Urgency: human.UrgencyHigh,
		}
		return Result{WaitingForHuman: true, Human: &h, Execution: Execution{RefusalReason: placed.Reason}}, nil
	}

	if wrote, _, err := bootstrap.EnsureCursorRules(dest, sel); err != nil {
		return Result{Execution: Execution{RefusalReason: "cursor rules: " + err.Error()}}, nil
	} else {
		_ = wrote
	}
	_ = apprun.Save(dest, apprun.ForType(sel.Type))

	res, err := e.Prepare(ctx, Request{ProductRoot: dest, PRDPath: filepath.Join(dest, "PRD.md"), PRDOnly: true})
	if err != nil {
		return res, err
	}
	res.ProjectType = sel.Type
	res.ProjectLocation = dest
	if res.Execution.ProductRoot == "" {
		res.Execution.ProductRoot = dest
	}
	e.persistBootstrapMeta(dest, abs, sel, dest)
	if res.WaitingForHuman {
		return res, nil
	}
	if res.Execution.RefusalReason == "" {
		_ = e.ensureRulesetForRoot(ctx, dest, doc)
	}
	return res, nil
}

func (e *Engine) persistBootstrapMeta(root, sourcePRD string, sel bootstrap.Selection, dest string) {
	store, err := state.Open(root)
	if err != nil {
		return
	}
	g, err := store.Lock()
	if err != nil {
		return
	}
	defer func() { _ = g.Unlock() }()
	st, err := g.Load()
	if err != nil {
		return
	}
	st.PRDPath = filepath.Join(root, "PRD.md")
	st.ProjectType = sel.Type
	st.ProjectLocation = dest
	st.StudioCategory = sel.Category
	st.Bootstrap.Status = state.BootstrapComplete
	st.Bootstrap.SourcePRD = sourcePRD
	st.Bootstrap.Destination = dest
	st.Bootstrap.GitDone = true
	st.Bootstrap.CursorRulesDone = true
	_ = g.AppendEvent(state.Event{
		Kind: state.KindResult,
		Name: state.EventProjectTypeSelected,
		Payload: state.Payload(map[string]string{
			"type": sel.Type, "category": sel.Category, "location": dest, "reason": sel.Reason,
		}),
	})
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventBootstrapCompleted,
		Payload: state.Payload(map[string]string{"root": dest, "source_prd": sourcePRD}),
	})
	_ = g.Save(st)
}

func (e *Engine) studioLayout() studio.Layout {
	cwd := e.opts.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	home := e.opts.Home
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	lay, _ := studio.Discover(e.cfg().StudioRoot, cwd, home)
	return lay
}
