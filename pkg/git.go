package pkg

import (
	"fmt"
	"io"

	"github.com/charmbracelet/log"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type CommitFile struct {
	FileName       string
	CurrentContent string
	PrevContent    string
}

type GitWrap interface {
	GetFiles() ([]*CommitFile, error)
	PopCommit() (string, error)
	CurrCommit() string
}

type GitWrapper struct {
	Commits []*object.Commit
	Idx     int
	Repo    *git.Repository
}

func NewGitWrapper() (*GitWrapper, error) {
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
		log.Errorf("Error iterating over Commits: %v", err)
		return nil, err
	}

	return &GitWrapper{
		Commits: revCommits,
		Idx:     0,
		Repo:    repo,
	}, nil
}

func (g *GitWrapper) PopCommit() (string, error) {
	if g.Idx+1 >= len(g.Commits) {
		return "", fmt.Errorf("no more Commits to pop")
	}

	g.Idx++
	return g.Commits[g.Idx].Hash.String(), nil
}

func (g *GitWrapper) CurrCommit() string {
	if g.Idx >= len(g.Commits) {
		return ""
	}
	return g.Commits[g.Idx].Hash.String()
}

func (g *GitWrapper) GetFiles() ([]*CommitFile, error) {
	if g.Idx >= len(g.Commits) {
		return nil, fmt.Errorf("commit index out of range")
	}

	commit := g.Commits[g.Idx]

	parentIter := commit.Parents()
	parent, err := parentIter.Next()
	if err != nil {
		if err == object.ErrParentNotFound {
			parent = nil
		} else {
			return nil, fmt.Errorf("failed to get parent commit: %v", err)
		}
	}

	var patch *object.Patch
	if parent != nil {
		patch, err = parent.Patch(commit)
		if err != nil {
			return nil, fmt.Errorf("failed to get patch: %v", err)
		}
	} else {
		patch, err = commit.Patch(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get initial patch: %v", err)
		}
	}

	commitFiles := []*CommitFile{}

	for _, filePatch := range patch.FilePatches() {
		from, to := filePatch.Files()

		var prevContent string
		var currentContent string
		var fileName string

		if from != nil {
			fileName = from.Path()
			file, err := parent.File(from.Path())
			if err != nil {
				prevContent = ""
			} else {
				content, err := file.Contents()
				if err != nil {
					return nil, fmt.Errorf("failed to get content of file %s from parent: %v", from.Path(), err)
				}
				prevContent = content
			}
		} else if to != nil {
			fileName = to.Path()
			prevContent = ""
		}

		if to != nil {
			fileName = to.Path()
			file, err := commit.File(to.Path())
			if err != nil {
				currentContent = ""
			} else {
				content, err := file.Contents()
				if err != nil {
					return nil, fmt.Errorf("failed to get content of file %s from commit: %v", to.Path(), err)
				}
				currentContent = content
			}
		} else if from != nil {
			fileName = from.Path()
			currentContent = ""
		}

		commitFiles = append(commitFiles, &CommitFile{
			FileName:       fileName,
			CurrentContent: currentContent,
			PrevContent:    prevContent,
		})
	}

	return commitFiles, nil
}
