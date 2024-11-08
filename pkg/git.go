package pkg

import (
	"io"

	"github.com/charmbracelet/log"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type PerdurantFile struct {
	FileName    string
	PastContent string
	Diff        *diff.FilePatch
}

type GitWrap interface {
	NextFile() (*PerdurantFile, error)
	PopCommit() (string, error)
	CurrCommit() string
}

type GitWrapper struct {
	commits []*object.Commit
	idx     int
	fileIdx int
	patch   *object.Patch
	files   map[string]*PerdurantFile
}

func NewGitWrapper() (GitWrap, error) {
	repo, err := git.PlainOpen("/Users/xaviermaruff/forwhy")
	if err != nil {
		log.Errorf("Failed to open repository: %v", err)
		return nil, err
	}

	ref, err := repo.Head()
	if err != nil {
		log.Errorf("Failed to get HEAD reference: %v", err)
		return nil, err
	}

	commitIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		log.Errorf("Failed to get commit history: %v", err)
		return nil, err
	}

	revCommits := make([]*object.Commit, 0)
	err = commitIter.ForEach(func(c *object.Commit) error {
		revCommits = append(revCommits, c)

		return nil
	})

	if err != nil && err != io.EOF {
		log.Errorf("Error iterating over commits: %v", err)
		return nil, err
	}

	return &GitWrapper{
		commits: revCommits,
		idx:     0,
		fileIdx: 0,
		files:   make(map[string]*PerdurantFile),
	}, nil
}

func (g *GitWrapper) PopCommit() (string, error) {
	g.idx++
	g.fileIdx = 0
	parents := g.commits[g.idx].Parents()
	parent, err := parents.Next()
	if err != nil {
		if err == object.ErrParentNotFound {
			return g.commits[g.idx].Hash.String(), nil
		}
		return "", err
	}

	g.patch, err = parent.Patch(g.commits[g.idx])
	if err != nil {
		return "", err
	}

	return g.commits[g.idx].Hash.String(), nil
}

func (g *GitWrapper) CurrCommit() string {
	return g.commits[g.idx].Hash.String()
}

func (g *GitWrapper) NextFile() (*PerdurantFile, error) {
	if g.patch == nil {
		if _, err := g.PopCommit(); err != nil {
			return nil, err
		}
	}

	if g.fileIdx >= len(g.patch.FilePatches()) {
		return nil, nil
	}

	filePatch := g.patch.FilePatches()[g.fileIdx]
	_, to := filePatch.Files()

	if to == nil {
		g.fileIdx++
		return g.NextFile()
	}

	fileRef := g.files[to.Path()]

	cont, err := g.commits[g.idx].File(to.Path())
	if err != nil {
		return nil, err
	}
	contVal, err := cont.Contents()
	if err != nil {
		return nil, err
	}

	if fileRef == nil {
		fileRef = &PerdurantFile{
			FileName:    to.Path(),
			PastContent: contVal,
			Diff:        &filePatch,
		}
		g.files[to.Path()] = fileRef
		g.fileIdx++

		return fileRef, nil
	}

	fileRef.PastContent, err = g.applyPatch(fileRef.PastContent, filePatch)
	if err != nil {
		return nil, err
	}
	fileRef.Diff = &filePatch
	g.fileIdx++

	return fileRef, nil
}

func (g *GitWrapper) applyPatch(content string, patch diff.FilePatch) (string, error) {
	_, toFile := patch.Files()
	if toFile == nil {
		return "", nil
	}

	currentCommit := g.commits[g.idx]
	file, err := currentCommit.File(toFile.Path())
	if err != nil {
		return content, err
	}
	reader, err := file.Blob.Reader()
	if err != nil {
		return content, err
	}
	defer reader.Close()
	newContentBytes, err := io.ReadAll(reader)
	if err != nil {
		return content, err
	}
	return string(newContentBytes), nil
}
