package translation

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/reader"
)

func TestTranslationInlineEquivalentTagsAndEscapedText(t *testing.T) {
	link := strings.Repeat("a", 64)
	source := `<strong>Read <a href="#reader-link" data-reader-link="` + link + `">the guide</a>.</strong> Example &amp;#x672C;.`
	raw := `<b>请阅读<a data-reader-link="` + link + `" href="#reader-link">指南</a>。</b>&amp;#x6839;据示例 &amp;#x672C;。&amp;#xA0;&amp;#60;`
	got, ok := validateUnit(translationUnit{HTML: source}, raw)
	if !ok {
		t.Fatal("equivalent emphasis and attribute order should validate")
	}
	if !strings.Contains(got, `<strong>请阅读<a href="#reader-link" data-reader-link="`+link+`">指南</a>。</strong>根据示例 &amp;#x672C;。`) ||
		!strings.Contains(got, "\u00a0") || !strings.Contains(got, "&amp;#60;") {
		t.Fatalf("unsafe or incomplete text normalization: %s", got)
	}
	if _, ok := validateUnit(translationUnit{HTML: source}, strings.ReplaceAll(raw, link, strings.Repeat("b", 64))); ok {
		t.Fatal("a changed link token must not validate")
	}
}

func TestTranslationRepairsRejectedFormattingWithoutRepeatingGoodBlocks(t *testing.T) {
	_, store, id := phase1Article(t, "<p><strong>A section heading.</strong> Read the explanation.</p><p>An ordinary paragraph.</p>")
	model := &fakeModel{fn: func(_ context.Context, request *ai.ModelRequest, call int32) (*ai.ModelResponse, error) {
		payload := readPhase1Payload(t, request)
		switch call {
		case 1:
			if len(payload.Items) != 2 {
				t.Fatalf("first request: %+v", payload.Items)
			}
			payload.Items[0].Text = "**章节标题。** 阅读说明。"
			payload.Items[1].Text = "普通段落。"
		case 2:
			if len(payload.Items) != 1 || payload.Items[0].ID != "b0" ||
				!strings.Contains(payload.Items[0].Text, "[[FT0]]") || !strings.Contains(request.System, "方括号标记") {
				t.Fatalf("repair request: %+v", payload.Items)
			}
			payload.Items[0].Text = "[[FT0]]章节标题。[[FT1]] &#x6839;据说明。"
		default:
			t.Fatal("unexpected extra retries")
		}
		return phase1ModelReply(t, payload, "stop"), nil
	}}
	svc := New(store, fakeResolver{model: model}, reader.NewService(store, nil, nil, nil), nil, nil)
	out, err := svc.Translate(context.Background(), TranslateRequest{ArticleID: id, SourceMode: "feed", TargetLang: "zh-CN"})
	if err != nil || out.Status != "completed" || len(out.Blocks) != 2 ||
		out.Blocks[0].TranslatedHTML != "<strong>章节标题。</strong> 根据说明。" || out.Blocks[1].TranslatedHTML != "普通段落。" {
		t.Fatalf("format repair left an incomplete article: %+v %v", out, err)
	}

	// An existing saved translation also gets cleaned on read, without a provider request.
	out.Blocks[0].TranslatedHTML = "<strong>章节标题。</strong> &amp;#x6839;据说明。"
	if err := svc.saveRecord(context.Background(), out, sql.NullInt64{}, sql.NullInt64{}); err != nil {
		t.Fatal(err)
	}
	cached, err := svc.Get(context.Background(), id, "zh-CN", "feed")
	if err != nil || cached.Blocks[0].TranslatedHTML != "<strong>章节标题。</strong> 根据说明。" || model.calls.Load() != 2 {
		t.Fatalf("legacy stored escaped text was not cleaned: %+v %v", cached, err)
	}
}

func TestTranslationRepairRejectsInvalidMarkers(t *testing.T) {
	unit := translationUnit{HTML: "<strong>Title.</strong>"}
	unit.format = encodeInline(unit.HTML)
	for _, bad := range []string{"[[FT0]]标题。", "[[FT1]]标题。[[FT0]]", "[[FT0]]标题。[[FT0]][[FT1]]", "[[FT0]]标题。[[FT2]][[FT1]]"} {
		if _, ok := validateUnit(unit, bad); ok {
			t.Errorf("malformed markers were accepted: %q", bad)
		}
	}
}

func TestTranslationMovesWholeCodeMarkersWithinTheirGroup(t *testing.T) {
	planner := batchPlanner{model: "test", system: "Translate.", budget: ai.ContextBudget(ai.Metadata{})}
	source := SourceBlock{ID: "b0", InnerHTML: `I added a <code>PreToolUse</code> hook in <code>.agents/hooks.json</code>.`}
	plan, err := planner.split(source)
	if err != nil || len(plan.units) != 1 {
		t.Fatalf("plan for inline code: %+v %v", plan, err)
	}
	unit := plan.units[0]
	if got := unit.wireText(); got != "I added a [[CODE0]] hook in [[CODE1]]." {
		t.Fatalf("inline code must use one opaque marker per span: %q", got)
	}
	want := `我在 <code>.agents/hooks.json</code> 中添加了一个 <code>PreToolUse</code> 钩子。`
	if got, ok := validateUnit(unit, "我在 [[CODE1]] 中添加了一个 [[CODE0]] 钩子。"); !ok || got != want {
		t.Fatalf("natural Chinese order should survive: %q %v", got, ok)
	}
	for _, bad := range []string{"[[CODE1]] [[CODE1]]", "[[CODE1]] [[CODE2]]", "[[CODE0]]"} {
		if _, ok := validateUnit(unit, bad); ok {
			t.Fatalf("accepted missing or replaced code: %q", bad)
		}
	}
	protected := `A <code>first</code> value and <strong>a <code>second</code> value</strong>.`
	plan, err = planner.split(SourceBlock{ID: "b1", InnerHTML: protected})
	if err != nil || len(plan.units) != 1 {
		t.Fatalf("plan for nested code: %+v %v", plan, err)
	}
	if _, ok := validateUnit(plan.units[0], "[[CODE1]] 和 [[FT1]][[CODE0]][[FT3]]"); ok {
		t.Fatal("code must stay within its surrounding formatting markers")
	}
}
