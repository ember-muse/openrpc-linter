package cmd

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestSkillPrintsMarkdown(t *testing.T) {
	want, err := os.ReadFile("SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	command := newRootCommand()
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"--skill"})
	err = command.Execute()
	if err != nil {
		t.Fatal(err)
	}
	if len(want) == 0 || !bytes.Equal(stdout.Bytes(), want) || stderr.Len() != 0 {
		t.Fatalf("expected only the skill file on stdout; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRootHelp(t *testing.T) {
	for _, args := range [][]string{nil, {"--skill=false"}, {"--help"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			command := newRootCommand()
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetArgs(args)
			err := command.Execute()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), "Usage:") || !strings.Contains(output.String(), "--skill") {
				t.Fatalf("expected help advertising --skill, got %q", output.String())
			}
		})
	}
}

type failingWriter struct {
	err error
}

func (w failingWriter) Write(p []byte) (int, error) {
	return 0, w.err
}

func TestSkillReturnsWriteError(t *testing.T) {
	want := errors.New("output unavailable")
	command := newRootCommand()
	command.SetOut(failingWriter{err: want})
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{"--skill"})
	err := command.Execute()
	if !errors.Is(err, want) {
		t.Fatalf("expected write error %v, got %v", want, err)
	}
}
