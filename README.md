# 🎥 gitanimate

Turn your git history into typing animations. Not very useful, but fun!

## Installation

Ensure you have raylib and ffmpeg installed.
Then:

```bash
go install github.com/Xavier-Maruff/gitanimate@latest
```

## Usage

```bash
gitanimate /path/to/repo
```

Every modified file in every commit (or in the specified range, `gitanimate help` for details)
then gets rendered into a video, typing out the changes, with syntax highlighting, line numbers,
a cursor, and configurable theme.
