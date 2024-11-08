package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/Xavier-Maruff/gitanimate/pkg"
	"github.com/charmbracelet/log"
	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

var (
	font       rl.Font
	style              = styles.Get("catppuccin-mocha")
	fontSize   float32 = 20
	lineHeight float32 = fontSize * 1.2
)

func main() {
	rand.Seed(time.Now().UnixNano())
	raylibMain()
}

func tokenizeCode(lang string, code string) ([]chroma.Token, error) {
	log.Infof("Tokenizing code with language: %s", lang)
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

func raylibMain() {
	rl.SetConfigFlags(rl.FlagWindowResizable | rl.FlagVsyncHint | rl.FlagWindowHighdpi)
	rl.InitWindow(1080, 700, "gitanimate")
	defer rl.CloseWindow()

	rl.SetTargetFPS(60)
	bg := style.Get(chroma.Background)
	bgRl := rl.Color{R: bg.Background.Red(), G: bg.Background.Green(), B: bg.Background.Blue(), A: 255}

	loadFonts()
	g, err := pkg.NewGitWrapper()
	if err != nil {
		log.Fatal(err)
	}

	_, err = g.PopCommit()

	files, err := g.GetFiles()
	if err != nil {
		log.Fatal(err)
	}

	code := files[0]

	tokens, err := tokenizeCode(lang(code.FileName), code.CurrentContent)
	if err != nil {
		log.Fatal(err)
	}

	var typedCharsCount int
	var cursorVisible bool = true
	var cursorTimer float32
	var scrollOffsetY float32

	var nextCharTimer float32
	var minDelay float32 = 0.001
	var maxDelay float32 = 0.17

	nextCharTimer = random(minDelay, maxDelay)

	var totalChars int
	for _, token := range tokens {
		totalChars += len(token.Value)
	}

	for !rl.WindowShouldClose() {
		deltaTime := rl.GetFrameTime()

		cursorTimer += deltaTime
		if cursorTimer >= 0.5 {
			cursorVisible = !cursorVisible
			cursorTimer = 0
		}

		if typedCharsCount < totalChars {
			nextCharTimer -= deltaTime
			if nextCharTimer <= 0 {
				typedCharsCount++
				nextCharTimer = random(minDelay, maxDelay)
			}
		}

		rl.BeginDrawing()
		rl.ClearBackground(bgRl)

		_, cursorY := renderTokens(tokens, 10, 10, typedCharsCount, cursorVisible, scrollOffsetY)

		windowHeight := float32(rl.GetScreenHeight())
		linesFromBottom := lineHeight * 2

		if cursorY-scrollOffsetY+linesFromBottom > windowHeight {
			scrollOffsetY = cursorY + linesFromBottom - windowHeight
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
	case "go":
		return "go"
	case "md":
		return "markdown"
	case "f90", "f95":
		return "systemverilog"
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
