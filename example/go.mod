module github.com/snormore/go-e2e/example

go 1.24.3

require (
	github.com/codeglyph/go-dotignore v1.0.2 // indirect
	github.com/snormore/go-e2e v0.0.0-00010101000000-000000000000 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/snormore/go-e2e => ../

tool github.com/snormore/go-e2e/cmd/go-e2e
