package gitanimate

import (
	"embed"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/reiver/go-whitespace"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"

	"github.com/schollz/progressbar/v3"
	"github.com/sergi/go-diff/diffmatchpatch"
)

type AnimateParams struct {
	Output   string
	Font     string
	Theme    string
	MinDelay float32
	MaxDelay float32
	Width    int32
	Height   int32
}

type AnimateDiffParams struct {
	Diffs          []diffmatchpatch.Diff
	PrevContent    string
	Filename       string
	Params         *AnimateParams
	UpdateProgress func(tea.Msg) (tea.Model, tea.Cmd)
	ShowWindow     bool
	Pos            int
	Total          int
}

type AnimState struct {
	OpIndex        int
	CharIndex      int
	Diffs          []diffmatchpatch.Diff
	Lang           string
	Filename       string
	UpdateProgress func(tea.Msg) (tea.Model, tea.Cmd)
}

var (
	//go:embed assets
	fonts      embed.FS
	font       rl.Font
	style              = styles.Get("catppuccin-mocha")
	fontSize   float32 = 20
	lineHeight float32 = fontSize * 1.2
	Logger             = log.NewWithOptions(os.Stderr, log.Options{
		//ReportCaller: true,
		//Prefix:       "[gitanimate]",
	})
	LangExts = map[string]string{
		"f90":  "fortran",
		"f?":   "fortran",
		"c":    "c",
		"cpp":  "cpp",
		"py":   "python",
		"sh":   "bash",
		"js":   "javascript",
		"html": "html",
		"css":  "css",
		"java": "java",
		"rs":   "rust",
		"go":   "go",
		"md":   "markdown",
	}
)

const (
	FrameRate     = 10
	FFmpegPath    = "ffmpeg"
	FFmpegPreset  = "veryslow"
	MaxFrameCount = 1000
	FrameFormat   = "frame_%04d.png"
)

func tokenizeCode(lang string, code string) ([]chroma.Token, error) {
	lexer := lexers.Get(lang)
	if lexer == nil {
		return nil, fmt.Errorf("no lexer found for %s", lang)
	}
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return nil, err
	}
	tokens := []chroma.Token{}
	for token := iterator(); token != chroma.EOF; token = iterator() {
		tokens = append(tokens, token)
	}
	return tokens, nil
}

func loadFont(name string) {
	if name == "default" {
		f, _ := fonts.ReadFile("assets/fonts/HurmitNerdFontMono-Regular.otf")
		font = rl.LoadFontFromMemory(".otf", f, 120, nil)
	} else {
		font = rl.LoadFontEx(name, 120, nil, 0)
	}
}

func getColorForTokenType(tokenType chroma.TokenType) rl.Color {
	entry := style.Get(tokenType)
	color := entry.Colour
	return rl.Color{R: color.Red(), G: color.Green(), B: color.Blue(), A: 255}
}

func renderTokens(tokens []chroma.Token, startX, startY float32, typedCharsCount int, cursorVisible bool, scrollOffsetY float32) (float32, float32) {
	lineNumberWidth := float32(50)
	x, y := startX+lineNumberWidth, startY
	charsRendered := 0
	var cursorX, cursorY float32 = x, y
	lineNumber := 1

	screenWidth := float32(rl.GetScreenWidth())

	lineNumberStr := fmt.Sprintf("%d", lineNumber)
	rl.DrawTextEx(font, lineNumberStr, rl.Vector2{X: startX, Y: y - scrollOffsetY}, fontSize, 0, rl.Gray)

	for _, token := range tokens {
		color := getColorForTokenType(token.Type)
		text := token.Value
		for _, char := range text {
			if charsRendered >= typedCharsCount {
				break
			}
			charStr := string(char)
			if char == '\n' {
				x = startX + lineNumberWidth
				y += lineHeight
				lineNumber++
				lineNumberStr := fmt.Sprintf("%d", lineNumber)
				rl.DrawTextEx(font, lineNumberStr, rl.Vector2{X: startX, Y: y - scrollOffsetY}, fontSize, 0, rl.Gray)
			} else {
				charWidth := rl.MeasureTextEx(font, charStr, fontSize, 0).X

				if x+charWidth > screenWidth-10 {
					x = startX + lineNumberWidth
					y += lineHeight
					lineNumber++
					lineNumberStr := fmt.Sprintf("%d", lineNumber)
					rl.DrawTextEx(font, lineNumberStr, rl.Vector2{X: startX, Y: y - scrollOffsetY}, fontSize, 0, rl.Gray)
				}

				rl.DrawTextEx(font, charStr, rl.Vector2{X: x, Y: y - scrollOffsetY}, fontSize, 0, color)
				x += charWidth
			}
			charsRendered++
			cursorX = x
			cursorY = y
		}
		if charsRendered >= typedCharsCount {
			break
		}
	}

	if cursorVisible {
		rl.DrawRectangle(int32(cursorX), int32(cursorY-scrollOffsetY), 2, int32(lineHeight), rl.White)
	}

	return cursorX, cursorY
}

func renderTokensAll(tokens []chroma.Token, startX, startY float32, cursorVisible bool, scrollOffsetY float32, cursorIndex int) (float32, float32) {
	lineNumberWidth := float32(50)
	x, y := startX+lineNumberWidth, startY
	charsRendered := 0
	var cursorX, cursorY float32 = x, y
	lineNumber := 1

	screenWidth := float32(rl.GetScreenWidth())

	lineNumberStr := fmt.Sprintf("%d", lineNumber)
	rl.DrawTextEx(font, lineNumberStr, rl.Vector2{X: startX, Y: y - scrollOffsetY}, fontSize, 0, rl.Gray)

	currIdx := 0
	for _, token := range tokens {
		color := getColorForTokenType(token.Type)
		text := token.Value
		for i, char := range text {
			charStr := string(char)
			if char == '\n' {
				x = startX + lineNumberWidth
				y += lineHeight
				lineNumber++
				lineNumberStr := fmt.Sprintf("%d", lineNumber)
				rl.DrawTextEx(font, lineNumberStr, rl.Vector2{X: startX, Y: y - scrollOffsetY}, fontSize, 0, rl.Gray)
			} else {
				charWidth := rl.MeasureTextEx(font, charStr, fontSize, 0).X

				if x+charWidth > screenWidth-10 {
					x = startX + lineNumberWidth
					y += lineHeight
					lineNumberStr := ""
					rl.DrawTextEx(font, lineNumberStr, rl.Vector2{X: startX, Y: y - scrollOffsetY}, fontSize, 0, rl.Gray)
				}

				rl.DrawTextEx(font, charStr, rl.Vector2{X: x, Y: y - scrollOffsetY}, fontSize, 0, color)
				x += charWidth
			}
			charsRendered++
			if currIdx+i == cursorIndex {
				cursorX = x
				cursorY = y
			}
		}
		currIdx += len(text)
	}

	if cursorVisible {
		rl.DrawRectangle(int32(cursorX), int32(cursorY-scrollOffsetY), 2, int32(lineHeight/1.4), rl.White)
	}

	return cursorX, cursorY
}

func (a *AnimState) incr() bool {
	updateProgress := func() {
		if a.UpdateProgress != nil {
			a.UpdateProgress(float64(a.OpIndex) / float64(len(a.Diffs)))
		}
	}
	defer updateProgress()

	for {
		if a.OpIndex >= len(a.Diffs)-1 {
			return true
		}

		if a.Diffs[a.OpIndex].Type != diffmatchpatch.DiffEqual {
			break
		}

		a.OpIndex++
	}

	a.CharIndex++
	if a.CharIndex >= len(a.Diffs[a.OpIndex].Text) {
		a.CharIndex = 0
		for {
			a.OpIndex++
			if a.OpIndex >= len(a.Diffs)-1 {
				return true
			}

			if a.Diffs[a.OpIndex].Type != diffmatchpatch.DiffEqual {
				return false
			}

		}
	}

	return false
}

func (a *AnimState) renderTokens() ([]chroma.Token, int) {
	if a.OpIndex >= len(a.Diffs) {
		a.OpIndex = len(a.Diffs) - 1
	}

	cursorIndex := 0
	str := ""
	//prior character-wise changes
	for i := 0; i < a.OpIndex; i++ {
		switch a.Diffs[i].Type {
		case diffmatchpatch.DiffEqual:
			str += a.Diffs[i].Text
		case diffmatchpatch.DiffInsert:
			str += a.Diffs[i].Text
		}
	}

	if a.OpIndex >= len(a.Diffs) {
		tokens, err := tokenizeCode(a.Lang, str)
		if err != nil {
			Logger.Fatal(err)
		}

		return tokens, len(str)
	}

	//current character-wise changes
	switch a.Diffs[a.OpIndex].Type {
	case diffmatchpatch.DiffInsert:
		//jumps whitespace at 2x speed
		if whitespace.IsWhitespace(rune(a.Diffs[a.OpIndex].Text[a.CharIndex])) {
			a.CharIndex++
			if a.CharIndex >= len(a.Diffs[a.OpIndex].Text) {
				a.CharIndex--
			}
			if random(0, 1) > 0.7 {
				time.Sleep(time.Duration(random(200, 500)) * time.Millisecond)
			}
		}

		cursorIndex = len(str) + a.CharIndex
		str += a.Diffs[a.OpIndex].Text[:a.CharIndex]
		if a.Diffs[a.OpIndex].Text[len(a.Diffs[a.OpIndex].Text)-1] == byte("\n"[0]) {
			str += "\n"
			cursorIndex -= 1
		}
	case diffmatchpatch.DiffDelete:
		//jump deletes word-wise
		if whitespace.IsWhitespace(rune(a.Diffs[a.OpIndex].Text[a.CharIndex])) {
			for a.CharIndex < len(a.Diffs[a.OpIndex].Text) && whitespace.IsWhitespace(rune(a.Diffs[a.OpIndex].Text[a.CharIndex])) {
				a.CharIndex++
			}
		}

		lines := strings.Split(a.Diffs[a.OpIndex].Text, "\n")
		if len(lines) > 3 {
			//delete in line chunks
			cursorIndex = len(str)
			a.CharIndex = a.CharIndex + len(lines[0]) + 1
			str = str[:len(str)-a.CharIndex]
			break
		}

		cursorIndex = len(str) - a.CharIndex
		str = str[:len(str)-a.CharIndex]

		if cursorIndex >= len(str) {
			cursorIndex = len(str) - 1
		}

		if str[cursorIndex] == "\n"[0] {
			cursorIndex = max(cursorIndex-1, 0)
		}
	case diffmatchpatch.DiffEqual:
		cursorIndex = len(str)
		a.CharIndex = len(a.Diffs[a.OpIndex].Text)
		str += a.Diffs[a.OpIndex].Text
	}

	//no change post current op index
	for i := a.OpIndex + 1; i < len(a.Diffs); i++ {
		switch a.Diffs[i].Type {
		case diffmatchpatch.DiffEqual:
			str += a.Diffs[i].Text
		case diffmatchpatch.DiffDelete:
			str += a.Diffs[i].Text
		}
	}

	tokens, err := tokenizeCode(a.Lang, str)
	if err != nil {
		Logger.Fatal(err)
	}

	return tokens, cursorIndex
}

func AnimateDiff(params *AnimateDiffParams) error {
	err := os.MkdirAll(params.Params.Output, os.ModePerm)

	temp, err := os.MkdirTemp("", "gitanimate")
	if err != nil {
		Logger.Fatal(err)
	}
	defer os.RemoveAll(temp)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		<-c
		Logger.Info("Cleaning up temporary files")
		err := os.RemoveAll(temp)
		if err != nil {
			Logger.Errorf("Failed to clean up temporary files: %v", err)
		}

		os.Exit(1)
	}()

	bar := progressbar.NewOptions(len(params.Diffs),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetWidth(45),
		progressbar.OptionSetDescription(params.Filename+fmt.Sprintf(" (%d/%d) ", params.Pos, params.Total)),
	)

	rl.SetTraceLogLevel(rl.LogError)
	var flags uint32 = rl.FlagVsyncHint | rl.FlagWindowHighdpi
	if !params.ShowWindow {
		flags |= rl.FlagWindowHidden
	}
	rl.SetConfigFlags(flags)
	rl.InitWindow(params.Params.Width, params.Params.Height, "gitanimate")
	defer rl.CloseWindow()

	//renderTexture := rl.LoadRenderTexture(ScreenWidth, ScreenHeight)
	//defer rl.UnloadRenderTexture(renderTexture)

	rl.SetTargetFPS(FrameRate)

	style = styles.Get(params.Params.Theme)
	bg := style.Get(chroma.Background)
	bgRl := rl.Color{R: bg.Background.Red(), G: bg.Background.Green(), B: bg.Background.Blue(), A: 255}

	loadFont(params.Params.Font)

	var scrollOffsetY float32

	var nextCharTimer float32
	var minDelay float32 = params.Params.MinDelay
	var maxDelay float32 = params.Params.MaxDelay

	nextCharTimer = random(minDelay, maxDelay)

	tokens, err := tokenizeCode(lang(params.Filename), params.PrevContent)
	if err != nil {
		Logger.Fatal(err)
	}

	state := AnimState{
		OpIndex:        0,
		CharIndex:      0,
		Diffs:          params.Diffs,
		Lang:           lang(params.Filename),
		Filename:       params.Filename,
		UpdateProgress: params.UpdateProgress,
	}

	cursorIndex := 0
	frameCount := 0

	done := false
	postDone := 0

	for !rl.WindowShouldClose() {
		if done {
			postDone++
		}

		deltaTime := rl.GetFrameTime()

		nextCharTimer -= deltaTime
		if nextCharTimer <= 0 {
			done = state.incr()

			bar.Set(state.OpIndex)

			tokens, cursorIndex = state.renderTokens()
			nextCharTimer = float32(math.Exp(float64(random(-100, 0))))*(maxDelay-minDelay) + minDelay
			if nextCharTimer > 0.8*maxDelay {
				nextCharTimer = maxDelay
			} else {
				nextCharTimer = random(minDelay, nextCharTimer)
			}

			//nextCharTimer = random(minDelay, maxDelay)
		}

		rl.BeginDrawing()
		//rl.BeginTextureMode(renderTexture)
		rl.ClearBackground(bgRl)

		_, cursorY := renderTokensAll(tokens, 10, 10, true, scrollOffsetY, cursorIndex)

		windowHeight := float32(rl.GetScreenHeight())
		linesFromBottom := lineHeight * 2

		if cursorY-scrollOffsetY+linesFromBottom > windowHeight {
			scrollOffsetY = cursorY + linesFromBottom - windowHeight + windowHeight/2
		}

		//rl.EndTextureMode()
		rl.EndDrawing()

		img := rl.LoadImageFromScreen()
		//img := rl.LoadImageFromTexture(renderTexture.Texture)
		if img == nil {
			Logger.Fatal("Failed to load image from screen")
		}

		//rl.ImageFlipVertical(img)

		imgPath := filepath.Join(temp, fmt.Sprintf(FrameFormat, frameCount))
		if !rl.ExportImage(*img, imgPath) {
			rl.UnloadImage(img)
			Logger.Fatal(err)
		}
		rl.UnloadImage(img)

		//add extra frames at end to catch any missed changes
		if postDone > FrameRate/2 || frameCount > MaxFrameCount {
			break
		}

		frameCount++

		//time.Sleep(time.Second / FrameRate)
	}

	if err := encodeFramesToVideo(
		temp, frameCount+10,
		path.Join(params.Params.Output, strings.ReplaceAll(params.Filename, "/", "_"))+".mp4",
	); err != nil {
		Logger.Fatal(err)
	}

	bar.Set(len(params.Diffs))
	bar.Clear()
	return nil
}

func encodeFramesToVideo(temp string, frameCount int, output string) error {
	if frameCount == 0 {
		return fmt.Errorf("no frames captured to encode")
	}

	inputPattern := filepath.Join(temp, FrameFormat)

	cmd := exec.Command(
		FFmpegPath,
		"-y",
		"-framerate", fmt.Sprintf("%d", FrameRate),
		"-i", inputPattern,
		"-c:v", "libx264",
		"-preset", FFmpegPreset,
		"-crf", "18",
		"-pix_fmt", "yuv420p",
		output,
	)

	//cmd.Stdout = os.Stdout
	//cmd.Stderr = os.Stderr

	//Logger.Debug("Starting FFmpeg encoding...")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("FFmpeg error: %v", err)
	}

	return nil
}

func lang(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) < 2 {
		return "text"
	}
	ret := LangExts[parts[len(parts)-1]]
	if ret == "" {
		return "text"
	}

	return ret
}

func random(min, max float32) float32 {
	return min + rand.Float32()*(max-min)
}
