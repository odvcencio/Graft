package entity

import (
	"fmt"
	"sync"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

var pooledParseMu sync.Mutex

func parseFilePooled(filename string, source []byte) (*gotreesitter.BoundTree, error) {
	// gotreesitter v0.22.5 still has shared GLR forest fast-path state that is
	// visible to the race detector under concurrent ParseFilePooled callers.
	pooledParseMu.Lock()
	defer pooledParseMu.Unlock()
	return grammars.ParseFilePooled(filename, source)
}

func parseFile(filename string, source []byte, timeoutMicros uint64) (*gotreesitter.BoundTree, error) {
	if timeoutMicros == 0 {
		return parseFilePooled(filename, source)
	}
	entry := grammars.DetectLanguage(filename)
	if entry == nil {
		return nil, fmt.Errorf("unsupported file type: %s", filename)
	}
	lang := entry.Language()
	if lang == nil {
		return nil, fmt.Errorf("grammar %q is unavailable", entry.Name)
	}
	parser := gotreesitter.NewParser(lang)
	parser.SetTimeoutMicros(timeoutMicros)
	var (
		tree *gotreesitter.Tree
		err  error
	)
	if entry.TokenSourceFactory == nil {
		tree, err = parser.Parse(source)
	} else {
		tree, err = parser.ParseWithTokenSource(source, entry.TokenSourceFactory(source, lang))
	}
	if err != nil {
		if tree != nil {
			tree.Release()
		}
		return nil, err
	}
	if tree == nil {
		return nil, fmt.Errorf("parser returned no tree")
	}
	if tree.ParseStoppedEarly() {
		reason := tree.ParseStopReason()
		tree.Release()
		return nil, fmt.Errorf("parse stopped before accepting input: %s", reason)
	}
	return gotreesitter.Bind(tree), nil
}
