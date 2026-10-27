package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestLaunchGameEngine(t *testing.T) {
	svc := GitService{}

	t.Run("UnknownEngine", func(t *testing.T) {
		msg, err := svc.LaunchGameEngine(t.TempDir(), "cryengine")
		if err == nil {
			t.Fatalf("expected error for unknown engine, got success message %q", msg)
		}
		if !strings.Contains(err.Error(), "未知游戏引擎") {
			t.Fatalf("expected unknown engine error, got %v", err)
		}
		if msg != "" {
			t.Fatalf("expected empty message on failure, got %q", msg)
		}
	})

	t.Run("UnrealWithoutUProject", func(t *testing.T) {
		repo := t.TempDir()
		msg, err := svc.LaunchGameEngine(repo, "unreal")
		if err == nil {
			t.Fatalf("expected error when no .uproject exists, got success message %q", msg)
		}
		if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
			// The project check must short-circuit before any launcher runs, so
			// the failure is the detection error rather than a launch error.
			if !strings.Contains(err.Error(), "uproject") {
				t.Fatalf("expected .uproject detection error, got %v", err)
			}
		}
		if msg != "" {
			t.Fatalf("expected empty message on failure, got %q", msg)
		}
	})

	t.Run("GodotMissingProjectOnDarwin", func(t *testing.T) {
		if runtime.GOOS != "darwin" {
			t.Skip("open(1) failure semantics are macOS specific")
		}
		repo := t.TempDir() // project.godot intentionally absent
		msg, err := svc.LaunchGameEngine(repo, "godot")
		if err == nil {
			t.Fatalf("expected launch failure for missing project.godot, got success message %q", msg)
		}
		if !strings.Contains(err.Error(), "启动 Godot 失败") {
			t.Fatalf("expected Godot launch error, got %v", err)
		}
		if msg != "" {
			t.Fatalf("expected empty message on failure, got %q", msg)
		}
	})

	t.Run("UnityUnsupportedPlatform", func(t *testing.T) {
		if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
			t.Skip("launcher is wired up on this platform")
		}
		repo := t.TempDir()
		msg, err := svc.LaunchGameEngine(repo, "unity")
		if err == nil {
			t.Fatalf("expected unsupported platform error, got success message %q", msg)
		}
		if !strings.Contains(err.Error(), "当前平台暂不支持") {
			t.Fatalf("expected unsupported platform error, got %v", err)
		}
		if msg != "" {
			t.Fatalf("expected empty message on failure, got %q", msg)
		}
	})
}
