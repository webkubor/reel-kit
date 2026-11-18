package main

import (
	"strings"
	"testing"
)

func TestSceneReferenceSelectionAndWorkspace(t *testing.T) {
	d := SubjectDetail{Subject: Subject{Name: "雨夜长街", Kind: "scene", WorkspaceID: "snow"}, Assets: []SubjectAsset{
		{Role: "raw", URL: "https://example.com/draft.png"},
		{Role: "standard", URL: "https://example.com/video.mp4", MediaType: "video"},
		{Role: "standard", URL: "https://example.com/scene.png", MediaType: "image"},
	}}
	ref, err := sceneReference(d, "snow")
	if err != nil || ref.URL != "https://example.com/scene.png" {
		t.Fatalf("选错参考图: %+v %v", ref, err)
	}
	if !strings.Contains(ref.Label, "不作为人物") {
		t.Fatal("缺少场景参考的用途限制")
	}
	for _, ws := range []string{"", "other"} {
		if _, err := sceneReference(d, ws); err == nil {
			t.Fatal("未拒绝跨项目参考")
		}
	}
	d.Assets = d.Assets[:2]
	if _, err := sceneReference(d, "snow"); err == nil {
		t.Fatal("没有标准图片时不应静默退化为文字参考")
	}
}

func TestH3SceneReferenceBudgetAndPrompt(t *testing.T) {
	refs := []shotRef{}
	for i := 0; i < 9; i++ {
		refs = append(refs, shotRef{URL: "character", Label: "人物定妆"})
	}
	refs = withSceneReference(refs, shotRef{URL: "scene", Label: "雨夜长街场景，仅用于空间"})
	if len(refs) != 9 || refs[0].URL != "character" || refs[8].URL != "scene" {
		t.Fatalf("参考预算与顺序错误: %+v", refs)
	}
	prompt := h3ShotPrompt(&Character{Name: "夜惊鸿"}, "", "回望", shotSpec{Ratio: "16:9", Duration: 6}, refs, "")
	if !strings.Contains(prompt, "图9") || !strings.Contains(prompt, "雨夜长街场景") {
		t.Fatal("没有把场景图编号与用途写进实际出镜提示词")
	}
}
