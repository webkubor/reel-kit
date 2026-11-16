package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateSubjectPlainIDReceipt(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "museav")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf '3f38d8c8-ad41-408e-b075-a3fde1474618\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	client := &MuseavClient{Bin: bin}
	s, err := client.CreateSubject(context.Background(), "角色", "person", "real", map[string]string{"发型": "长发"})
	if err != nil {
		t.Fatal(err)
	}
	if s.ID != "3f38d8c8-ad41-408e-b075-a3fde1474618" || s.Traits["发型"] != "长发" {
		t.Fatalf("unexpected subject: %+v", s)
	}
}

func TestCharacterPromptsAreSingleBatchLines(t *testing.T) {
	s := &Subject{Traits: map[string]any{"发型": "长发"}}
	for _, view := range CharacterViews {
		p := characterViewPrompt(s, CharacterCreateInput{}, view)
		for _, r := range p {
			if r == '\n' || r == '\r' {
				t.Fatalf("%s prompt contains line break", view)
			}
		}
	}
}
