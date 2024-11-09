package main

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/Xavier-Maruff/gitanimate/pkg"
	"github.com/charmbracelet/log"
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/reiver/go-whitespace"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"

	"github.com/sergi/go-diff/diffmatchpatch"
)

var (
	font       rl.Font
	style              = styles.Get("catppuccin-mocha")
	fontSize   float32 = 20
	lineHeight float32 = fontSize * 1.2
)

func main() {
	raylibMain()
}

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

func loadFonts() {
	font = rl.LoadFontEx("assets/fonts/HurmitNerdFontMono-Regular.otf", 120, nil, 0)
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

type AnimState struct {
	OpIndex   int
	CharIndex int
	Diffs     []diffmatchpatch.Diff
	Lang      string
	Filename  string
}

func (a *AnimState) incr() {
	for {
		if a.OpIndex >= len(a.Diffs)-1 {
			break
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
				return
			}

			if a.Diffs[a.OpIndex].Type != diffmatchpatch.DiffEqual {
				return
			}

		}
	}
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
			log.Fatal(err)
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

		if str[cursorIndex] == '\n' {
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
		log.Fatal(err)
	}

	return tokens, cursorIndex
}

func raylibMain() {
	rl.SetConfigFlags(rl.FlagWindowResizable | rl.FlagVsyncHint | rl.FlagWindowHighdpi)
	rl.InitWindow(800, 900, "gitanimate")
	defer rl.CloseWindow()

	rl.SetTargetFPS(60)
	bg := style.Get(chroma.Background)
	bgRl := rl.Color{R: bg.Background.Red(), G: bg.Background.Green(), B: bg.Background.Blue(), A: 255}

	loadFonts()
	g, err := pkg.NewGitWrapper()
	if err != nil {
		log.Fatal(err)
	}

	for i := 0; i < 5; i++ {
		_, err = g.PopCommit()
	}

	files, err := g.GetFiles()
	if err != nil {
		log.Fatal(err)
	}

	code := files[1]
	tokens, err := tokenizeCode(lang(code.FileName), code.PrevContent)
	if err != nil {
		log.Fatal(err)
	}

	diffs := diffmatchpatch.New().DiffMain(code.PrevContent, code.CurrentContent, false)
	diffs = diffmatchpatch.New().DiffCleanupSemanticLossless(diffs)
	diffs = diffmatchpatch.New().DiffCleanupMerge(diffs)

	//var typedCharsCount int
	//var cursorVisible bool = true
	//var cursorTimer float32
	var scrollOffsetY float32

	var nextCharTimer float32
	var minDelay float32 = 0.01
	var maxDelay float32 = 0.05

	nextCharTimer = random(minDelay, maxDelay)

	state := AnimState{
		OpIndex:   0,
		CharIndex: 0,
		Diffs:     diffs,
		Lang:      lang(code.FileName),
		Filename:  code.FileName,
	}

	cursorIndex := 0
	for !rl.WindowShouldClose() {
		deltaTime := rl.GetFrameTime()

		/*cursorTimer += deltaTime
		if cursorTimer >= 0.5 {
			cursorVisible = !cursorVisible
			cursorTimer = 0
			}*/

		nextCharTimer -= deltaTime
		if nextCharTimer <= 0 {
			//cursorVisible = true
			if diffs[state.OpIndex].Type != diffmatchpatch.DiffEqual {
				//log.Infof("Op: %s [%d], Char: %s [%d]", diffs[state.OpIndex].Type, state.OpIndex, string(diffs[state.OpIndex].Text[state.CharIndex]), state.CharIndex)
			}
			state.incr()
			tokens, cursorIndex = state.renderTokens()
			nextCharTimer = random(minDelay, maxDelay)
		}

		rl.BeginDrawing()
		rl.ClearBackground(bgRl)

		_, cursorY := renderTokensAll(tokens, 10, 10, true, scrollOffsetY, cursorIndex)

		windowHeight := float32(rl.GetScreenHeight())
		linesFromBottom := lineHeight * 2

		if cursorY-scrollOffsetY+linesFromBottom > windowHeight {
			scrollOffsetY = cursorY + linesFromBottom - windowHeight + windowHeight/2
		}

		rl.EndDrawing()
	}
}

func lang(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) < 2 {
		return "text"
	}
	ext := parts[len(parts)-1]
	switch ext {
	case "go", "f?":
		return "go"
	case "md":
		return "markdown"
	case "f90", "f95":
		return "fortran"
	case "c":
		return "c"
	default:
		log.Warnf("Unknown language for file: %s", filename)
		return "text"
	}
}

func random(min, max float32) float32 {
	return min + rand.Float32()*(max-min)
}
