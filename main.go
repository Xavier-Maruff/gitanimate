package main

import (
	"path"

	gitanimate "github.com/Xavier-Maruff/gitanimate/pkg"
	"github.com/charmbracelet/log"

	"github.com/sergi/go-diff/diffmatchpatch"
)

func main() {
	outputDir := "output"
	g, err := gitanimate.NewGitWrapper()
	if err != nil {
		log.Fatal(err)
	}

	for i := 0; i < 8; i++ {
		_, err = g.PopCommit()
	}

	localOut := path.Join(outputDir, "1")

	files, err := g.GetFiles()
	if err != nil {
		log.Fatal(err)
	}

	code := files[1]

	diffs := diffmatchpatch.New().DiffMain(code.PrevContent, code.CurrentContent, false)
	diffs = diffmatchpatch.New().DiffCleanupSemanticLossless(diffs)
	diffs = diffmatchpatch.New().DiffCleanupMerge(diffs)

	err = gitanimate.AnimateDiff(diffs, code.PrevContent, localOut, code.FileName)
	if err != nil {
		log.Fatal(err)
	}
}
