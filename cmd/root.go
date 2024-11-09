package cmd

import (
	"os"
	"path"
	"strconv"

	gitanimate "github.com/Xavier-Maruff/gitanimate/pkg"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gitanimate <repo_path> [flags]",
	Short: "Create typewriter animations from git repos",
	Long:  ``,
	Run:   runGitAnimate,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringP("output", "o", "", "Path to output directory")
	rootCmd.Flags().StringP("font", "f", "default", "Font to use")
	rootCmd.Flags().StringP("theme", "t", "default", "Theme to use")
	rootCmd.Flags().Float32P("max_delay", "s", 1, "Maximum delay between edits")
	rootCmd.Flags().Float32P("min_delay", "i", 0.01, "Minimum delay between edits")
	rootCmd.Flags().StringP("start", "a", "initial", "Commit to start from")
	rootCmd.Flags().StringP("end", "e", "", "Commit to end at")
	rootCmd.Flags().Int32P("max_commits", "m", 0, "Maximum number of commits to process")
	rootCmd.Flags().BoolP("show", "w", false, "Show the animation as it is created")
	rootCmd.Flags().Int32P("width", "x", 600, "Width of the output")
	rootCmd.Flags().Int32P("height", "y", 800, "Height of the output")
}

func runGitAnimate(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		gitanimate.Logger.Errorf("No repository path provided")
		gitanimate.Logger.Infof("Usage: gitanimate <repo_path>")
	}

	animParams := parseParams(cmd)
	repoPath := args[0]

	start, _ := cmd.Flags().GetString("start")
	end, _ := cmd.Flags().GetString("end")
	maxCommits, _ := cmd.Flags().GetInt32("max_commits")
	showWindow, _ := cmd.Flags().GetBool("show")

	output := animParams.Output

	gw, err := gitanimate.NewGitWrapper(repoPath, start, end)
	if err != nil {
		gitanimate.Logger.Fatalf("Failed to create GitWrapper: %v", err)
	}

	if maxCommits > 0 {
		gw.Commits = gw.Commits[:maxCommits]
	}

	i := 1
	for {
		gitanimate.Logger.Infof("Processing commit: %s (%d/%d)", gw.CurrCommit(), gw.Idx+1, len(gw.Commits))
		animParams.Output = path.Join(output, strconv.Itoa(i)+"_"+gw.CurrCommit()[:12])

		files, err := gw.GetFiles()
		if err != nil {
			gitanimate.Logger.Fatalf("Failed to get files from commit %s: %v", gw.CurrCommit(), err)
		}

		for i, f := range files {
			gitanimate.Logger.Infof("\t(%d/%d) File: %s", i+1, len(files), f.FileName)

			diffs := diffmatchpatch.New().DiffMain(f.PrevContent, f.CurrentContent, false)
			diffs = diffmatchpatch.New().DiffCleanupSemanticLossless(diffs)
			diffs = diffmatchpatch.New().DiffCleanupMerge(diffs)

			err := gitanimate.AnimateDiff(&gitanimate.AnimateDiffParams{
				Diffs:       diffs,
				PrevContent: f.PrevContent,
				Filename:    f.FileName,
				Params:      animParams,
				ShowWindow:  showWindow,
			})
			if err != nil {
				gitanimate.Logger.Errorf("Failed to animate diff: %v", err)
			}
		}

		_, err = gw.PopCommit()
		if err != nil {
			break
		}
	}

	gitanimate.Logger.Infof("All commits processed")
}

func parseParams(cmd *cobra.Command) *gitanimate.AnimateParams {
	outputDir, _ := cmd.Flags().GetString("output")
	font, _ := cmd.Flags().GetString("font")
	theme, _ := cmd.Flags().GetString("theme")
	minDelay, _ := cmd.Flags().GetFloat32("min_delay")
	maxDelay, _ := cmd.Flags().GetFloat32("max_delay")
	width, _ := cmd.Flags().GetInt32("width")
	height, _ := cmd.Flags().GetInt32("height")

	return &gitanimate.AnimateParams{
		Output:   outputDir,
		Font:     font,
		Theme:    theme,
		MinDelay: minDelay,
		MaxDelay: maxDelay,
		Width:    width,
		Height:   height,
	}
}
