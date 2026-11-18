package main

import (
	"fmt"
	"strings"
)

// withSceneReference 人物图在前，为场景留出 H3 的最后一个参考位。
func withSceneReference(refs []shotRef, scene shotRef) []shotRef {
	if len(refs) > 8 {
		refs = refs[:8]
	}
	return append(refs, scene)
}

// sceneReference 只采用场景主体下的正式标准图，不把原始草稿或输出视频当参考。
func sceneReference(detail SubjectDetail, workspace string) (shotRef, error) {
	if detail.Subject.Kind != "scene" || workspace == "" || detail.Subject.WorkspaceID != workspace {
		return shotRef{}, fmt.Errorf("场景「%s」不属于当前项目绑定的场景库", detail.Subject.Name)
	}
	for _, asset := range detail.Assets {
		if asset.Role != "standard" || !strings.HasPrefix(asset.URL, "https://") {
			continue
		}
		if asset.MediaType != "" && asset.MediaType != "image" && !strings.HasPrefix(asset.MediaType, "image/") {
			continue
		}
		return shotRef{URL: asset.URL, Label: detail.Subject.Name + "的场景参考，仅锁定建筑结构、地形、材质、空间布局与光线；不作为人物、服装或兵器参考，人物以之前的定妆图为准"}, nil
	}
	return shotRef{}, fmt.Errorf("场景「%s」没有正式参考图，请先上传标准场景图", detail.Subject.Name)
}
